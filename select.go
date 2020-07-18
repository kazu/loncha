package loncha

import (
	"errors"
	"reflect"
	"sort"
)

var (
	ERR_SLICE_TYPE           error = errors.New("parameter must be slice or pointer of slice")
	ERR_POINTER_SLICE_TYPE   error = errors.New("parameter must be pointer of slice")
	ERR_NOT_FOUND            error = errors.New("data is not found")
	ERR_ELEMENT_INVALID_TYPE error = errors.New("slice element is invalid type")
)

type CondFunc func(idx int) bool
type CompareFunc func(i, j int) bool

func slice2Reflect(slice interface{}) (reflect.Value, error) {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Ptr {
		return reflect.ValueOf(nil), ERR_SLICE_TYPE
	}

	if rv.Elem().Kind() != reflect.Slice {
		return reflect.ValueOf(nil), ERR_SLICE_TYPE
	}
	return rv, nil
}

func sliceElm2Reflect(slice interface{}) (reflect.Value, error) {
	rv := reflect.ValueOf(slice)

	if rv.Kind() == reflect.Slice {
		return rv, nil
	}

	if rv.Kind() != reflect.Ptr {
		return reflect.ValueOf(nil), ERR_POINTER_SLICE_TYPE
	}

	if rv.Elem().Kind() != reflect.Slice {
		return reflect.ValueOf(nil), ERR_SLICE_TYPE
	}
	return rv.Elem(), nil
}

// Reverse ... Transforms an array such that the first element will become the last, the second element will become the second to last, etc.
func Reverse(slice interface{}) {

	sort.Slice(slice, func(i, j int) bool { return i > j })
}

// Select ... return all element on match of CondFunc
func Select(slice interface{}, fn CondFunc) (interface{}, error) {

	rv, err := sliceElm2Reflect(slice)

	if err != nil {
		return nil, err
	}
	newSlice := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Cap())

	reflect.Copy(newSlice, rv)

	ptr := reflect.New(newSlice.Type())
	ptr.Elem().Set(newSlice)

	filter(ptr, fn)

	return ptr.Elem().Interface(), nil
}

// Filter ... Filter element with mached funcs
func Filter(slice interface{}, funcs ...CondFunc) error {

	rv, err := slice2Reflect(slice)
	if err != nil {
		return err
	}

	filter(rv, funcs...)
	return nil
}

// Delete ... Delete element with mached funcs
func Delete(slice interface{}, funcs ...CondFunc) error {
	rv, err := slice2Reflect(slice)
	if err != nil {
		return err
	}

	innterFilter(rv, false, funcs...)
	return nil
}

func filter(pRv reflect.Value, funcs ...CondFunc) {

	innterFilter(pRv, true, funcs...)
}

func innterFilter(pRv reflect.Value, keep bool, funcs ...CondFunc) {

	rv := pRv.Elem()

	length := rv.Len()
	if length == 0 {
		return
	}

	movelist := make([]int, length)
	newIdx := 0

	for i := 0; i < length; i++ {
		allok := (true == keep)
		for _, f := range funcs {
			if !f(i) {
				allok = (false == keep)
			}
		}

		if allok {
			movelist[i] = newIdx
			newIdx++
		} else {
			movelist[i] = -1
		}
	}

	if newIdx == length {
		return
	}

	swap := reflect.Swapper(pRv.Elem().Interface())

	for i, v := range movelist {
		if v != -1 {
			swap(i, v)
		}
	}

	pRv.Elem().SetLen(newIdx)

}
