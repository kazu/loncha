package loncha

type opParam[T any] struct {
	Param T
}

type Opt[T any] func(*opParam[T]) Opt[T]

func (p *opParam[T]) Options(opts ...Opt[T]) (prevs []Opt[T]) {

	for _, opt := range opts {
		prevs = append(prevs, opt(p))
	}
	return
}

func DefauiltOpt[T any]() *opParam[T] {
	return &opParam[T]{}
}

func MergeOpts[T any](opts ...Opt[T]) (*opParam[T], func(p *opParam[T])) {

	param := DefauiltOpt[T]()
	prevs := param.Options(opts...)

	return param, func(p *opParam[T]) {
		p.Options(prevs...)
	}
}
