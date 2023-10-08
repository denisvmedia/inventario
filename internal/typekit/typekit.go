package typekit

import (
	"reflect"
)

func NewOfType[T any](t T) T {
	ptr := reflect.New(reflect.TypeOf(t).Elem())
	elem := ptr.Interface()
	return elem.(T)
}
