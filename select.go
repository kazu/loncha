package loncha

import (
	"errors"
	"reflect"
)

var (
	ERR_SLICE_TYPE error = errors.New("parameter must be pointer of slice")
	ERR_NOT_FOUND  error = errors.New("data is not found")
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
		return reflect.ValueOf(nil), ERR_SLICE_TYPE
	}

	if rv.Elem().Kind() != reflect.Slice {
		return reflect.ValueOf(nil), ERR_SLICE_TYPE
	}
	return rv.Elem(), nil
}

func Select(slice interface{}, fn CondFunc) (interface{}, error) {

	rv, err := slice2Reflect(slice)
	if err != nil {
		return nil, err
	}
	newSlice := reflect.MakeSlice(rv.Elem().Type(), rv.Elem().Len(), rv.Elem().Cap())

	reflect.Copy(newSlice, rv.Elem())

	ptr := reflect.New(newSlice.Type())
	ptr.Elem().Set(newSlice)

	filter(ptr, fn)

	return ptr.Elem().Interface(), nil
}

func Filter(slice interface{}, funcs ...CondFunc) error {

	rv, err := slice2Reflect(slice)
	if err != nil {
		return err
	}

	filter(rv, funcs...)
	return nil
}

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
