package loncha

type ConvFunc[T any, S any] func(T) (S, bool)

func Conv[S, D any](srcs []S, convfn ConvFunc[S, D]) (dsts []D) {

	return Convertable(convfn)(srcs)
}
