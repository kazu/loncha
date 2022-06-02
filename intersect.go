//go:build go1.18
// +build go1.18

package loncha

import "sort"

func Intersect[T comparable](slice1, slice2 []T, opts ...Opt) (result []T) {

	param, fn := MergeOpts(opts...)
	defer fn(param)

	exists := map[T]bool{}
	already := map[T]bool{}

	for _, v := range slice1 {
		exists[v] = true
	}

	result = make([]T, 0, len(exists))

	for _, v := range slice2 {
		if param.Uniq && already[v] {
			continue
		}

		if exists[v] {
			result = append(result, v)
			already[v] = true
		}
	}
	return
}

type KeyFunc[T any, V comparable] func(slice []T, i int) V

func IntersectSorted[T any, V comparable](slice1, slice2 []T, fn KeyFunc[T, V]) (result []T) {

	for i, v := range slice1 {
		key := fn(slice1, i)
		idx := sort.Search(len(slice2), func(j int) bool {
			return fn(slice2, j) == key
		})
		if idx < len(slice2) && fn(slice2, idx) == key {
			result = append(result, v)
		}
	}
	return result
}
