package loncha

// InjectFn ... function for Inject()
type InjectFn[T any, R any] func(R, T) R

// Inject ... return an object formed from operands via InjectFn
func Inject[T any, V any](s []T, injectFn InjectFn[T, V]) (v V) {

	for _, t := range s {
		v = injectFn(v, t)
	}
	return
}

// Reduce ... alias of Inject
func Reduce[T any, V any](s []T, injectFn InjectFn[T, V]) (v V) {
	return Inject(s, injectFn)
}

// SumIdent ... return Ordered value  onnot-Ordered type
type SumIdent[T any, V Ordered] func(e T) V

// Sum ... sum of slice values in Ordered type
func Sum[T Ordered](s []T) (v T) {

	return Inject(s, func(result T, e T) T {
		return result + e
	})

}

// SumWithFn ... sum of slice values in non-Ordered type with SumIdent()
func SumWithFn[T any, V Ordered](s []T, fn SumIdent[T, V]) (v V) {
	return Inject(s, func(result V, e T) V {
		v := result + fn(e)
		return v
	})
}
