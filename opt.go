package loncha

type fnParam struct {
	CompareFn  CompareFunc
	CondFn     CompareFunc
	IsSort     bool
	SortBefore bool
	Uniq       bool
}

type Opt func(*fnParam) Opt

func (p *fnParam) Options(opts ...Opt) (prevs []Opt) {

	for _, opt := range opts {
		prevs = append(prevs, opt(p))
	}
	return
}

func DefauiltOpt() *fnParam {
	return &fnParam{Uniq: true}
}

func MergeOpts(opts ...Opt) (*fnParam, func(*fnParam)) {

	param := DefauiltOpt()
	prevs := param.Options(opts...)

	return param, func(p *fnParam) {
		p.Options(prevs...)
	}

}
