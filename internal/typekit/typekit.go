package typekit

func ZeroOfType[T any](T) (zero T) {
	// ptr := reflect.New(reflect.TypeOf(t).Elem())
	// elem := ptr.Interface()
	// return elem.(T)
	return zero
}
