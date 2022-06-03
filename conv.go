package loncha

type ConvFunc[T any, S any] func(T) (S, bool)

func Conv[S any, D any](srcs []S, convfn ConvFunc[S, D]) (dsts []D) {
	dsts = make([]D, 0, len(srcs))

	for _, s := range srcs {
		if d, removed := convfn(s); !removed {
			dsts = append(dsts, d)
		}
	}
	return
}
