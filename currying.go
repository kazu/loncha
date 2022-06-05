package loncha

// FilterFunc ... function  generated by Filterlize()
type FilterFunc[T any] func([]T) []T

// Filterable ... generate filter function for slice
func Filterable[T comparable](fns ...CondFunc2[T]) FilterFunc[T] {
	return innerfilterlable(true, fns...)
}

// Deletable ... generate deleting function by fns Condition for slice
func Deletable[T comparable](fns ...CondFunc2[T]) FilterFunc[T] {
	return innerfilterlable(false, fns...)
}

// Selectable ... generate deleting function by fns Condition for slice
func Selectable[T comparable](fns ...CondFunc2[T]) FilterFunc[T] {
	return Filterable(fns...)
}

func innerfilterlable[T comparable](keep bool, fns ...CondFunc2[T]) FilterFunc[T] {
	return func(srcs []T) (dsts []T) {
		dsts = srcs
		innerFilter2(&dsts, keep, fns...)
		return
	}
}

type InjecterFunc[T any, R any] func([]T) R

// Injectable ... generate Inject functions
func Injectable[T any, V any](injectFn InjectFn[T, V]) InjecterFunc[T, V] {

	return func(src []T) (result V) {
		return Inject(src, injectFn)
	}
}

// Reducable ... alias of Injectable
func Reducable[T any, V any](injectFn InjectFn[T, V]) InjecterFunc[T, V] {
	return Injectable(injectFn)
}
