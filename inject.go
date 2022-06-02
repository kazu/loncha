package loncha

type InjectFn[T any, R any] func(*R, T) R

func Inject[T any, V any](s []T, injectFn InjectFn[T, V]) (v V) {

	for _, t := range s {
		v = injectFn(&v, t)
	}
	return
}

func Reduce[T any, V any](s []T, injectFn InjectFn[T, V]) (v V) {
	return Inject(s, injectFn)
}

type InjectFn2[R any] func(*R, int) R

func Inject2[T any, V any](s []T, injectFn2 InjectFn2[V]) (v V) {

	for i, _ := range s {
		v = injectFn2(&v, i)
	}
	return
}

type InjectFn3 func(r any, i int) any

func Inject3[T any, V any](s []T, injectFn func(any, int) any) (v V) {

	for i, _ := range s {
		v = injectFn(&v, i).(V)
	}
	return
}
