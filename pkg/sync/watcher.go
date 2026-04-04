package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 300 * time.Millisecond

// ChangeType describes a file system change type.
type ChangeType int

const (
	ChangeCreate ChangeType = iota
	ChangeModify
	ChangeDelete
)

// FileChange is a single file change relative to Watcher.root.
type FileChange struct {
	Path string
	Type ChangeType
}

// Watcher emits batched file changes for a directory.
type Watcher struct {
	root      string
	exclude   []string
	polling   bool
	excluder  *pathExcluder
	pollEvery time.Duration
}

// NewWatcher creates a watcher for root.
func NewWatcher(root string, exclude []string, polling bool) (*Watcher, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("watch root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("watch root must be a directory")
	}

	excluder, err := newPathExcluder(exclude)
	if err != nil {
		return nil, err
	}

	return &Watcher{
		root:      absRoot,
		exclude:   append([]string(nil), exclude...),
		polling:   polling,
		excluder:  excluder,
		pollEvery: 500 * time.Millisecond,
	}, nil
}

// Watch starts watching and returns a channel of debounced change batches.
func (w *Watcher) Watch(ctx context.Context) <-chan []FileChange {
	out := make(chan []FileChange)
	if w.polling {
		go w.pollLoop(ctx, out)
	} else {
		go w.fsnotifyLoop(ctx, out)
	}
	return out
}

func (w *Watcher) fsnotifyLoop(ctx context.Context, out chan<- []FileChange) {
	defer close(out)

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer fw.Close()

	_ = filepath.WalkDir(w.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if w.isExcluded(path, true) && path != w.root {
			return filepath.SkipDir
		}
		_ = fw.Add(path)
		return nil
	})

	pending := make(map[string]FileChange)
	var timer *time.Timer
	var timerCh <-chan time.Time

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(debounceWindow)
			timerCh = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(debounceWindow)
	}
	flush := func() {
		if len(pending) == 0 {
			return
		}
		batch := make([]FileChange, 0, len(pending))
		for _, change := range pending {
			batch = append(batch, change)
		}
		pending = make(map[string]FileChange)
		select {
		case out <- batch:
		case <-ctx.Done():
		}
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
		flush()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			if err != nil {
				// Keep watching on fsnotify errors.
				continue
			}
		case event, ok := <-fw.Events:
			if !ok {
				return
			}

			isDir := pathExistsAndIsDir(event.Name)
			if w.isExcluded(event.Name, isDir) {
				continue
			}

			if event.Op&fsnotify.Create != 0 && isDir {
				_ = filepath.WalkDir(event.Name, func(path string, d os.DirEntry, walkErr error) error {
					if walkErr != nil || !d.IsDir() {
						return nil
					}
					if w.isExcluded(path, true) && path != w.root {
						return filepath.SkipDir
					}
					_ = fw.Add(path)
					return nil
				})
			}

			change, ok := w.toFileChange(event)
			if !ok {
				continue
			}
			prev, exists := pending[change.Path]
			if exists {
				pending[change.Path] = mergeChangeTypes(prev, change)
			} else {
				pending[change.Path] = change
			}
			resetTimer()
		case <-timerCh:
			flush()
		}
	}
}

func (w *Watcher) pollLoop(ctx context.Context, out chan<- []FileChange) {
	defer close(out)

	prev, _ := w.snapshot()
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, err := w.snapshot()
			if err != nil {
				continue
			}
			changes := diffSnapshots(prev, next)
			if len(changes) > 0 {
				select {
				case out <- changes:
				case <-ctx.Done():
					return
				}
			}
			prev = next
		}
	}
}

func (w *Watcher) snapshot() (map[string]fileState, error) {
	result := make(map[string]fileState)
	err := filepath.WalkDir(w.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		isDir := d.IsDir()
		if w.isExcluded(path, isDir) && path != w.root {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		if isDir {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := w.rel(path)
		if err != nil {
			return nil
		}
		result[rel] = fileState{
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		}
		return nil
	})
	return result, err
}

type fileState struct {
	Size    int64
	ModTime int64
}

func diffSnapshots(prev, next map[string]fileState) []FileChange {
	changes := make([]FileChange, 0)
	for path, before := range prev {
		after, exists := next[path]
		if !exists {
			changes = append(changes, FileChange{Path: path, Type: ChangeDelete})
			continue
		}
		if before != after {
			changes = append(changes, FileChange{Path: path, Type: ChangeModify})
		}
	}
	for path := range next {
		if _, exists := prev[path]; !exists {
			changes = append(changes, FileChange{Path: path, Type: ChangeCreate})
		}
	}
	return changes
}

func (w *Watcher) toFileChange(event fsnotify.Event) (FileChange, bool) {
	rel, err := w.rel(event.Name)
	if err != nil || rel == "" || rel == "." {
		return FileChange{}, false
	}

	switch {
	case event.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
		return FileChange{Path: rel, Type: ChangeDelete}, true
	case event.Op&fsnotify.Create != 0:
		return FileChange{Path: rel, Type: ChangeCreate}, true
	case event.Op&(fsnotify.Write|fsnotify.Chmod) != 0:
		return FileChange{Path: rel, Type: ChangeModify}, true
	default:
		return FileChange{}, false
	}
}

func mergeChangeTypes(prev, next FileChange) FileChange {
	if next.Type == ChangeDelete {
		return next
	}
	if prev.Type == ChangeCreate && next.Type == ChangeModify {
		return prev
	}
	return next
}

func (w *Watcher) rel(path string) (string, error) {
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func pathExistsAndIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (w *Watcher) isExcluded(path string, isDir bool) bool {
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return false
	}
	return w.excluder.IsExcluded(rel, isDir)
}

type pathExcluder struct {
	rules []excludeRule
}

type excludeRule struct {
	pattern  string
	negated  bool
	anchored bool
	dirOnly  bool
}

func newPathExcluder(patterns []string) (*pathExcluder, error) {
	rules := make([]excludeRule, 0, len(patterns))
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			continue
		}

		rule := excludeRule{}
		if strings.HasPrefix(raw, "!") {
			rule.negated = true
			raw = strings.TrimPrefix(raw, "!")
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "/") {
			rule.anchored = true
			raw = strings.TrimPrefix(raw, "/")
		}
		if strings.HasSuffix(raw, "/") {
			rule.dirOnly = true
			raw = strings.TrimSuffix(raw, "/")
		}
		rule.pattern = filepath.ToSlash(raw)

		rules = append(rules, rule)
	}
	return &pathExcluder{rules: rules}, nil
}

func (e *pathExcluder) IsExcluded(rel string, isDir bool) bool {
	excluded := false
	for _, rule := range e.rules {
		if rule.matches(rel, isDir) {
			excluded = !rule.negated
		}
	}
	return excluded
}

func (r excludeRule) matches(rel string, isDir bool) bool {
	rel = filepath.ToSlash(rel)
	if r.dirOnly && !isDir {
		// For dir-only rules support descendants too.
		return strings.HasPrefix(rel+"/", r.pattern+"/")
	}
	if r.dirOnly && isDir && strings.HasPrefix(rel+"/", r.pattern+"/") {
		return true
	}

	if r.anchored {
		if !strings.ContainsAny(r.pattern, "*?[") {
			return rel == r.pattern || strings.HasPrefix(rel, r.pattern+"/")
		}
		return matchPattern(r.pattern, rel)
	}

	if !strings.Contains(r.pattern, "/") {
		if matchPattern(r.pattern, filepath.Base(rel)) {
			return true
		}
		return matchPattern("**/"+r.pattern, rel)
	}

	if matchPattern(r.pattern, rel) {
		return true
	}
	return matchPattern("**/"+r.pattern, rel)
}

func matchPattern(pattern, rel string) bool {
	ok, err := doublestar.PathMatch(pattern, rel)
	return err == nil && ok
}
