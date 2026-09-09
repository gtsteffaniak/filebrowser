package settings

import (
	"reflect"
	"strings"
)

// ShareDefaultsValueAtPath returns the value at a dot-path on ShareDefaults.
// Unlike JSON marshaling, zero-valued fields (e.g. allowModify=false) are included.
func ShareDefaultsValueAtPath(d ShareDefaults, path string) (interface{}, bool) {
	return valueAtStructJSONPath(reflect.ValueOf(d), path)
}

func valueAtStructJSONPath(v reflect.Value, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return nil, false
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return nil, false
		}
		field, ok := structFieldByJSONTag(v, part)
		if !ok {
			return nil, false
		}
		v = field
		if i == len(parts)-1 {
			return v.Interface(), true
		}
	}
	return nil, false
}

func structFieldByJSONTag(v reflect.Value, name string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		jsonTag := field.Tag.Get("json")
		tagName := strings.Split(jsonTag, ",")[0]
		if tagName == "" || tagName == "-" {
			tagName = strings.ToLower(field.Name[:1]) + field.Name[1:]
		}
		if tagName != name {
			continue
		}
		return v.Field(i), true
	}
	return reflect.Value{}, false
}
