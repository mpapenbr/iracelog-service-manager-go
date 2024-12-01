//nolint:whitespace,lll,funlen,unused,unparam // readability
package racestints

import "time"

type calcParamOption func(p *ExpertCalcParams)

func withRaceDur(arg time.Duration) calcParamOption {
	return func(p *ExpertCalcParams) {
		p.RaceDur = arg
	}
}

func withLC(arg int) calcParamOption {
	return func(p *ExpertCalcParams) {
		p.LC = arg
	}
}

func withLps(arg int) calcParamOption {
	return func(p *ExpertCalcParams) {
		p.Lps = arg
	}
}

func createCopy(p *ExpertCalcParams, opts ...calcParamOption) *ExpertCalcParams {
	ret := *p
	for _, opt := range opts {
		opt(&ret)
	}
	return &ret
}
