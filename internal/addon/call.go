package addon

import (
	"reflect"
)

// callSymbol invokes a Yaegi func value (or returns error for non-funcs).
// Yaegi symbols are reflect.Value under the hood via interp.
func callSymbol(sym any, args ...any) ([]any, error) {
	rv, ok := sym.(reflect.Value)
	if !ok {
		// interp.Value often exposes Interface(); try type assertion paths.
		type interfaceer interface{ Interface() any }
		if iv, ok := sym.(interfaceer); ok {
			return invoke(iv.Interface(), args...)
		}
		return invoke(sym, args...)
	}
	if rv.Kind() != reflect.Func {
		return nil, errNotFunc
	}
	return invokeFunc(rv, args...)
}

func invoke(target any, args ...any) ([]any, error) {
	if target == nil {
		return nil, errNotFunc
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Func {
		return nil, errNotFunc
	}
	return invokeFunc(rv, args...)
}

func invokeFunc(rv reflect.Value, args ...any) ([]any, error) {
	t := rv.Type()
	if t.NumIn() != len(args) {
		return nil, errArity
	}
	in := make([]reflect.Value, len(args))
	for i, a := range args {
		if a == nil {
			in[i] = reflect.Zero(t.In(i))
			continue
		}
		av := reflect.ValueOf(a)
		if av.Type().AssignableTo(t.In(i)) {
			in[i] = av
			continue
		}
		if av.Type().ConvertibleTo(t.In(i)) {
			in[i] = av.Convert(t.In(i))
			continue
		}
		return nil, errArgType
	}
	out := rv.Call(in)
	res := make([]any, len(out))
	for i, o := range out {
		res[i] = o.Interface()
	}
	return res, nil
}
