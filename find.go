// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package finder for slice.
//
// To Find from slice: (list is slice)
//   loncha.Find(&list, func(i int) {
//	 	return list[i].ID == 555
//}

package loncha

// Find is value of slice if fn is true. if slice is not pointer of slice or empty, return error
func Find(slice interface{}, fn CondFunc) (interface{}, error) {

	idx, err := IndexOf(slice, fn)
	if err != nil {
		return nil, err
	}

	rv, _ := sliceElm2Reflect(slice)

	return rv.Index(idx).Interface(), nil

}

// IndexOf gets the index at which the first match fn is true. if not found. return -1.
// return error if slice is not pointer of the slice.
func IndexOf(slice interface{}, fn CondFunc) (int, error) {

	rv, err := sliceElm2Reflect(slice)
	if err != nil {
		return -1, err
	}

	length := rv.Len()
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

// LastIndexOf gets the last index at which the last match fn is true. if not found. return -1.
// return error if slice is not pointer of the slice.
func LastIndexOf(slice interface{}, fn CondFunc) (int, error) {

	rv, err := sliceElm2Reflect(slice)
	if err != nil {
		return -1, err
	}

	length := rv.Len()
	if length == 0 {
		return -1, err
	}
	for i := length - 1; i >= 0; i-- {
		if fn(i) {
			return i, nil
		}
	}
	return -1, ERR_NOT_FOUND
}

// Contain get return true which fn condition is true.
func Contain(slice interface{}, fn CondFunc) bool {

	idx, err := IndexOf(slice, fn)
	if err != nil {
		return false
	}
	if idx < 0 {
		return false
	}

	return true

}
