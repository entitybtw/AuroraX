package addon

import "errors"

var (
	errNotFunc = errors.New("addon: symbol is not a function")
	errArity   = errors.New("addon: function arity mismatch")
	errArgType = errors.New("addon: argument type mismatch")
)
