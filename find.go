package lonacha

func Find(slice interface{}, fn CondFunc) (interface{}, error) {

	idx, err := IndexOf(slice, fn)
	if err != nil {
		return nil, err
	}

	rv, _ := slice2Reflect(slice)

	return rv.Elem().Index(idx).Interface(), nil

}

func IndexOf(slice interface{}, fn CondFunc) (int, error) {

	rv, err := slice2Reflect(slice)
	if err != nil {
		return -1, err
	}

	length := rv.Elem().Len()
	if length == 0 {
		return -1, err
	}
	for i := 0; i < length; i++ {
		if fn(i) {
			return i, nil
		}
	}
	return -1, ERR_NOT_FOUND
}
