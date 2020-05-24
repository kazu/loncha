// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package DeDuplication for slice.
//
// To Find from slice: (list is slice)
//   loncha.Uniq(&list, func(i,j int) {
//	 	return list[i].ID == list[j].ID
//}

package loncha

import (
	"reflect"
	"sort"
)

type IdentFn func(i int) interface{}

// Uniq is deduplicate using fn . if slice is not pointer of slice or empty, return error
func Uniq(slice interface{}, fn IdentFn) error {

	pRv, err := slice2Reflect(slice)
	if err != nil {
		return err
	}
	n := pRv.Elem().Len()

	exists := make(map[interface{}]bool, n)

	err = Filter(slice, func(i int) bool {
		if !exists[fn(i)] {
			exists[fn(i)] = true
			return true
		}
		return false
	})
	exists = nil
	return err
}

// Uniq2 is deduplicate using fn . if slice is not pointer of slice or empty, return error
func Uniq2(slice interface{}, fn CompareFunc) error {

	pRv, err := slice2Reflect(slice)
	if err != nil {
		return err
	}
	n := pRv.Elem().Len()

	swap := reflect.Swapper(pRv.Elem().Interface())

	uniqCnt := int(1)

	for i := 0; i < n; i++ {
		for j := 0; j < uniqCnt; j++ {
			if fn(i, j) {
				goto SKIP
			}
		}
		swap(uniqCnt, i)
		uniqCnt++
	SKIP:
	}
	pRv.Elem().SetLen(uniqCnt)

	return err
}

// UniqWithSort is deduplicating using fn, sorting before dedup. if slice is not pointer of slice or empty, return error
func UniqWithSort(slice interface{}, fn CompareFunc) (int, error) {

	pRv, err := slice2Reflect(slice)
	if err != nil {
		return -1, err
	}
	n := pRv.Elem().Len()

	if !sort.SliceIsSorted(pRv.Elem().Interface(), fn) {
		sort.Slice(pRv.Elem().Interface(), fn)
	}
	swap := reflect.Swapper(pRv.Elem().Interface())

	a, b := 0, 1
	for b < n {
		if fn(a, b) {
			a++
			if a != b {
				swap(a, b)
			}
		}
		b++
	}
	return a + 1, nil

}
