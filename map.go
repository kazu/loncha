package loncha

func Keys[K comparable, V any](m map[K]V) (keys []K) {

	if m == nil {
		return nil
	}

	keys = make([]K, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}
	return
}

func Values[K comparable, V any](m map[K]V) (values []V) {

	if m == nil {
		return nil
	}

	values = make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return
}
