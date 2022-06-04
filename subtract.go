package loncha

import "sort"

type subOpt struct {
}

func mapOfExists[T comparable](slice []T) (exists map[T]bool) {

	exists = map[T]bool{}
	for _, v := range slice {
		exists[v] = true
	}

	return
}

// Sub .. subtraction between two slices.
func Sub[T comparable](slice1, slice2 []T, opts ...Opt[subOpt]) (result []T) {

	param, fn := MergeOpts(opts...)
	defer fn(param)

	exists := mapOfExists(slice2)

	for _, v := range slice1 {
		if _, found := exists[v]; !found {
			result = append(result, v)
		}
	}
	return

}

// SubSorted ... subtraction in sorted slice
func SubSorted[T any, V Ordered](slice1, slice2 []T, IdentFn IdentFunc[T, V]) (result []T) {

	jn := 0
	result = make([]T, 0, len(slice2))
	for i, v := range slice1 {
		key := IdentFn(slice1, i)
		idx := sort.Search(len(slice2)-jn, func(j int) bool {
			return IdentFn(slice2, jn+j) >= key
		})
		_ = v
		if len(slice2) <= idx {
			result = append(result, v)
			break
		}

		// copy before idx -1
		if IdentFn(slice2, idx) != key {
			result = append(result, v)

		}
		// MENTION: required ?
		if IdentFn(slice2, idx) < key {
			jn = idx
		}

	}
	return result

}
