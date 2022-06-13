package loncha

import "reflect"

type CondFunc2[T any] func(t *T) bool

type CondFuncWithIndex[T any] func(i int, t *T) bool

// FilterOpt ... functional option for FIlter2()
type FilterOpt[T any] struct {
	isDestructive bool
	condFns       []CondFunc
	condFns2      []CondFunc2[T]
	fVersion      int
	equalObject   T
}

func (fopt *FilterOpt[T]) condFn(fns ...CondFunc) (prev []CondFunc) {

	prev = fopt.condFns
	fopt.condFns = fns
	return prev

}

func (fopt *FilterOpt[T]) condFn2(fns ...CondFunc2[T]) (prev []CondFunc2[T]) {

	prev = fopt.condFns2
	fopt.condFns2 = fns
	return prev
}

func (fopt *FilterOpt[T]) filterVersion(v int) (prev int) {
	prev = fopt.fVersion
	fopt.fVersion = v
	return prev
}

func (fopt *FilterOpt[T]) equal(v T) (prev T) {
	prev = fopt.equalObject
	fopt.equalObject = v
	return prev
}

type eqaulSetter[T any, S any] interface {
	equal(S) S
	*T
}

type fVersionSetter[T any] interface {
	filterVersion(int) int
	*T
}

type condFuncSetter[T any] interface {
	condFn(fns ...CondFunc) (prevs []CondFunc)
	*T
}

type condFuncSetter2[T any, S any] interface {
	condFn(fns ...CondFunc) (prevs []CondFunc)
	condFn2(fns ...CondFunc2[S]) (prevs []CondFunc2[S])
	*T
}

func FilterVersion[T any, PT fVersionSetter[T]](v int) Opt[T] {

	return func(p *opParam[T]) Opt[T] {
		prevs := PT(&p.Param).filterVersion(v)
		return FilterVersion[T, PT](prevs)
	}

}

// Cond ... set conditional function for FIlter2()
func Cond[T any, PT condFuncSetter[T]](fns ...CondFunc) Opt[T] {
	return func(p *opParam[T]) Opt[T] {
		prevs := PT(&p.Param).condFn(fns...)
		return Cond[T, PT](prevs...)
	}
}

// Cond2 ... no index variant of Cond
func Cond2[OptT any, S comparable, OptPT condFuncSetter2[OptT, S]](fns ...CondFunc2[S]) Opt[OptT] {
	return func(p *opParam[OptT]) Opt[OptT] {
		prevs := OptPT(&p.Param).condFn2(fns...)
		return Cond2[OptT, S, OptPT](prevs...)
	}
}

func Equal[T any, S any, PT eqaulSetter[T, S]](v S) Opt[T] {
	return func(p *opParam[T]) Opt[T] {
		prevs := PT(&p.Param).equal(v)
		return Equal[T, S, PT](prevs)
	}
}

// Filter ... FIlter implementation with type parameters
func Filter[T comparable](slice []T, condFn CondFunc2[T], opts ...Opt[FilterOpt[T]]) ([]T, error) {

	opt, prev := MergeOpts(opts...)
	defer prev(opt)

	eq := new(T)
	if opt.Param.equalObject != *eq {
		innerFilter2(&slice, true,
			func(t *T) bool {
				return *t == opt.Param.equalObject
			})
	}
	if condFn != nil {
		innerFilter2(&slice, true, condFn)
		return slice, nil
	}

	if len(opt.Param.condFns) > 0 {
		err := OldFilter(&slice, opt.Param.condFns...)
		return slice, err
	}
	if len(opt.Param.condFns2) > 0 {
		switch opt.Param.fVersion {
		case 3:
			innerFilter3(&slice, true, opt.Param.condFns2...)
		case 4:
			innerFilter4(&slice, true, opt.Param.condFns2...)
		default:
			innerFilter2(&slice, true, opt.Param.condFns2...)
		}
		return slice, nil
	}

	return slice, nil
}

func innerFilter2[T any](pslice *[]T, keep bool, funcs ...CondFunc2[T]) {

	length := len(*pslice)

	if length == 0 {
		return
	}

	movelist := make([]int, length)
	newIdx := 0

	for i := 0; i < length; i++ {
		allok := (true == keep)
		for _, f := range funcs {
			if !f(&(*pslice)[i]) {
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

	swap := reflect.Swapper(*pslice)

	for i, v := range movelist {
		if v != -1 {
			swap(i, v)
		}
	}

	(*pslice) = (*pslice)[:newIdx]

}

type filterRange struct {
	Start int
	End   int
}

func innerFilter4[T comparable](pslice *[]T, keep bool, funcs ...CondFunc2[T]) {

	slice := *pslice
	length := len(*pslice)

	if length == 0 {
		return
	}

	skiplist := make([]filterRange, 0, length)

	for i := 0; i < length; i++ {
		allok := (true == keep)
		for _, f := range funcs {
			if !f(&(*pslice)[i]) {
				allok = (false == keep)
			}
		}
		if allok {
			continue
		}

		if len(skiplist) == 0 { // add first
			skiplist = append(skiplist, filterRange{i, i})
			continue
		}

		if skiplist[len(skiplist)-1].End+1 == i {
			skiplist[len(skiplist)-1].End++
			continue
		}

		skiplist = append(skiplist, filterRange{i, i})
	}

	// not perged
	if len(skiplist) == length {
		return
	}

	slices := make([][]T, 0)
	for i, _ := range skiplist {
		v := skiplist[i]
		be := 0
		if i > 0 {
			be = skiplist[i-1].End + 1
		}
		if v.End == len(slice)-1 {
			pv := skiplist[i-1]
			slices = append(slices, slice[pv.End+1:v.Start:v.Start])

			continue
		}
		if be == 0 && v.Start == 0 {
			continue
		}
		if len(slices) == 0 {
			slices = append(slices, slice[be:v.Start])
			continue
		}
		slices = append(slices, slice[be:v.Start:v.Start])

	}
	slice = Inject(slices,
		func(old []T, e []T) (neo []T) {
			if len(old) == 0 {
				return e
			}
			neo = old
			neo = neo[:len(neo)+len(e)]
			copy(neo[len(old):], e)
			return neo
		})
	slices = nil
	*pslice = slice

}

func innerFilter3[T comparable](pslice *[]T, keep bool, funcs ...CondFunc2[T]) {

	slice := *pslice
	length := len(*pslice)

	if length == 0 {
		return
	}

	skiplist := make([]filterRange, 0, length)

	for i := 0; i < length; i++ {
		allok := (true == keep)
		for _, f := range funcs {
			if !f(&(*pslice)[i]) {
				allok = (false == keep)
			}
		}
		if allok {
			continue
		}

		if len(skiplist) == 0 { // add first
			skiplist = append(skiplist, filterRange{i, i})
			continue
		}

		if skiplist[len(skiplist)-1].End+1 == i {
			skiplist[len(skiplist)-1].End++
			continue
		}

		skiplist = append(skiplist, filterRange{i, i})
	}

	// not perged
	if len(skiplist) == length {
		return
	}

	purgeCnt := 0
	for i, _ := range skiplist {
		v := skiplist[i]
		end := v.End - purgeCnt + 1
		if end == len(slice) {
			slice = slice[0 : v.Start-purgeCnt]
			break
		}

		slice = append(slice[0:v.Start-purgeCnt], slice[end:]...)

		purgeCnt += v.End - v.Start + 1
	}

	*pslice = slice

}
