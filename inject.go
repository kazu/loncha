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
