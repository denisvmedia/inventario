package mapkit

func Clone[T comparable, S any](m map[T]S) map[T]S {
	result := make(map[T]S)
	for k, v := range m {
		result[k] = v
	}
	return result
}
