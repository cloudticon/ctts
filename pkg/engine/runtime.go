package engine

import (
	"fmt"

	"github.com/dop251/goja"
)

type Resource = map[string]interface{}

type ExecuteOpts struct {
	JSCode    string
	Values    map[string]interface{}
	Namespace string
}

func Execute(opts ExecuteOpts) ([]Resource, error) {
	vm := goja.New()

	if err := injectGlobals(vm, opts.Values); err != nil {
		return nil, fmt.Errorf("failed to inject globals: %w", err)
	}

	if _, err := vm.RunString(opts.JSCode); err != nil {
		return nil, fmt.Errorf("JS execution error: %w", err)
	}

	resources, err := extractResources(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to extract resources: %w", err)
	}

	postProcess(resources, opts.Namespace)
	return resources, nil
}

func injectGlobals(vm *goja.Runtime, values map[string]interface{}) error {
	global := vm.GlobalObject()

	registry := vm.NewArray()
	if err := global.Set("__ct_resources", registry); err != nil {
		return err
	}

	initScript := `
		if (typeof globalThis !== 'undefined') {
			globalThis.__ct_resources = __ct_resources;
		}
	`
	if _, err := vm.RunString(initScript); err != nil {
		return err
	}

	if values == nil {
		values = map[string]interface{}{}
	}
	valuesObj := vm.ToValue(values)
	if err := global.Set("Values", valuesObj); err != nil {
		return err
	}

	valuesInitScript := `
		if (typeof globalThis !== 'undefined') {
			globalThis.Values = Values;
		}
	`
	if _, err := vm.RunString(valuesInitScript); err != nil {
		return err
	}

	return nil
}

func extractResources(vm *goja.Runtime) ([]Resource, error) {
	val := vm.GlobalObject().Get("__ct_resources")
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, fmt.Errorf("__ct_resources is not defined")
	}

	exported := val.Export()
	arr, ok := exported.([]interface{})
	if !ok {
		return nil, fmt.Errorf("__ct_resources is not an array, got %T", exported)
	}

	resources := make([]Resource, 0, len(arr))
	for i, item := range arr {
		res, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("resource at index %d is not an object", i)
		}
		resources = append(resources, res)
	}

	return resources, nil
}

func postProcess(resources []Resource, namespace string) {
	for _, res := range resources {
		scope, _ := res["__ctts_scope"].(string)
		delete(res, "__ctts_scope")

		if scope != "cluster" && namespace != "" {
			applyNamespaceDefault(res, namespace)
		}

		cleanNilFields(res)
	}
}

func applyNamespaceDefault(res Resource, namespace string) {
	meta, ok := res["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	if ns, exists := meta["namespace"]; !exists || ns == nil || ns == "" {
		meta["namespace"] = namespace
	}
}

func cleanNilFields(obj map[string]interface{}) {
	for k, v := range obj {
		if v == nil {
			delete(obj, k)
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			cleanNilFields(val)
			if len(val) == 0 {
				delete(obj, k)
			}
		case []interface{}:
			cleaned := cleanNilSlice(val)
			if len(cleaned) == 0 {
				delete(obj, k)
			} else {
				obj[k] = cleaned
			}
		}
	}
}

func cleanNilSlice(arr []interface{}) []interface{} {
	result := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		if item == nil {
			continue
		}
		if m, ok := item.(map[string]interface{}); ok {
			cleanNilFields(m)
		}
		result = append(result, item)
	}
	return result
}
