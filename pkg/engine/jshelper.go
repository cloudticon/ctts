package engine

import (
	"fmt"
	"unicode"

	"github.com/dop251/goja"
)

// JSHelper simplifies registering Go functions as Goja globals.
// NOT goroutine-safe -- use one JSHelper per VM, single goroutine.
type JSHelper struct {
	vm *goja.Runtime
}

func NewJSHelper(vm *goja.Runtime) *JSHelper {
	return &JSHelper{vm: vm}
}

// DefineFunc registers fn as a JS global with automatic:
//   - error -> JS exception conversion
//   - panic recovery (non-goja panics wrapped as JS errors)
//   - nil return -> undefined
//   - globalThis alias
func (h *JSHelper) DefineFunc(name string, fn func(args *Args) (interface{}, error)) {
	if !IsValidJSIdentifier(name) {
		panic(fmt.Sprintf("invalid JS identifier: %q", name))
	}
	h.vm.Set(name, func(call goja.FunctionCall) (ret goja.Value) {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(*goja.Object); ok {
					panic(r) // re-throw Goja's own JS exceptions
				}
				panic(h.vm.NewGoError(fmt.Errorf("%s(): %v", name, r)))
			}
		}()
		args := &Args{vm: h.vm, call: call, fnName: name}
		result, err := fn(args)
		if err != nil {
			panic(h.vm.NewGoError(fmt.Errorf("%s(): %w", name, err)))
		}
		if result == nil {
			return goja.Undefined()
		}
		return h.vm.ToValue(result)
	})
	h.aliasGlobalThis(name)
}

// DefineValue registers a constant on the global scope.
func (h *JSHelper) DefineValue(name string, value interface{}) {
	if !IsValidJSIdentifier(name) {
		panic(fmt.Sprintf("invalid JS identifier: %q", name))
	}
	h.vm.Set(name, h.vm.ToValue(value))
	h.aliasGlobalThis(name)
}

// DefineArray creates an empty JS Array (with push/pop/etc.) on the global scope.
// Use instead of DefineValue for arrays that need JS Array methods.
func (h *JSHelper) DefineArray(name string) {
	if !IsValidJSIdentifier(name) {
		panic(fmt.Sprintf("invalid JS identifier: %q", name))
	}
	h.vm.Set(name, h.vm.NewArray())
	h.aliasGlobalThis(name)
}

func (h *JSHelper) aliasGlobalThis(name string) {
	h.vm.RunString(fmt.Sprintf(
		`if(typeof globalThis!=='undefined')globalThis.%s=%s;`, name, name,
	))
}

func IsValidJSIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) && r != '_' && r != '$' {
			return false
		}
		if i > 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' {
			return false
		}
	}
	return true
}
