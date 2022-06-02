//go:build go1.18
// +build go1.18

package loncha

import "sort"

// Intersect ... intersection between 2 slice.
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

// Ordered ... copy from https://github.com/golang/go/blob/go1.18.3/test/typeparam/ordered.go
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}

type IdentFunc[T any, V Ordered] func(slice []T, i int) V

// IntersectSorted ... intersection between 2 sorted slice
func IntersectSorted[T any, V Ordered](slice1, slice2 []T, IdentFn IdentFunc[T, V]) (result []T) {

	jn := 0
	for i, v := range slice1 {
		key := IdentFn(slice1, i)
		idx := sort.Search(len(slice2)-jn, func(j int) bool {
			return IdentFn(slice2, j+jn) >= key
		})
		if idx < len(slice2) && IdentFn(slice2, idx) == key {
			result = append(result, v)
			jn = idx
		}
	}
	return result
}
