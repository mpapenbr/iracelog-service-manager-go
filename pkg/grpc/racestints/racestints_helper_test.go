//nolint:whitespace,lll,funlen,unused,unparam,errcheck // readability
package racestints

import (
	"time"

	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

type (
	predictParamOption func(p *predictv1.PredictParam)
	raceParamOption    func(p *predictv1.RaceParam)
	carParamOption     func(p *predictv1.CarParam)
	pitParamOption     func(p *predictv1.PitParam)
	stintParamOption   func(p *predictv1.StintParam)
	fuelParamOption    func(p *predictv1.FuelParam)

	stintPartOption func(p *predictv1.StintPart)
	pitPartOption   func(p *predictv1.PitPart)
)

func CreateStintPart(start, duration time.Duration, partStint *predictv1.Part_Stint) *predictv1.Part {
	return &predictv1.Part{
		Start:    durationpb.New(start),
		End:      durationpb.New(start + duration),
		Duration: durationpb.New(duration),
		PartType: partStint,
	}
}

func CreatePitPart(start, duration time.Duration, partPit *predictv1.Part_Pit) *predictv1.Part {
	return &predictv1.Part{
		Start:    durationpb.New(start),
		End:      durationpb.New(start + duration),
		Duration: durationpb.New(duration),
		PartType: partPit,
	}
}

func conv(dur string) time.Duration {
	//nolint:errcheck // we know the format is correct
	d, _ := time.ParseDuration(dur)
	return d
}

func createPartStint(laps, startLap, endLap int32) *predictv1.Part_Stint {
	return &predictv1.Part_Stint{Stint: &predictv1.StintPart{
		Laps: laps, LapStart: startLap, LapEnd: endLap,
	}}
}

func createPartPit() *predictv1.Part_Pit {
	return &predictv1.Part_Pit{Pit: &predictv1.PitPart{}}
}

func CopyPredictParam(p *predictv1.PredictParam, opts ...predictParamOption) *predictv1.PredictParam {
	ret := proto.Clone(p).(*predictv1.PredictParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithPPRace(arg *predictv1.RaceParam) predictParamOption {
	return func(p *predictv1.PredictParam) {
		p.Race = arg
	}
}

func WithPPCar(arg *predictv1.CarParam) predictParamOption {
	return func(p *predictv1.PredictParam) {
		p.Car = arg
	}
}

func WithPPPit(arg *predictv1.PitParam) predictParamOption {
	return func(p *predictv1.PredictParam) {
		p.Pit = arg
	}
}

func WithPPStint(arg *predictv1.StintParam) predictParamOption {
	return func(p *predictv1.PredictParam) {
		p.Stint = arg
	}
}

func WithPPFuel(arg *predictv1.FuelParam) predictParamOption {
	return func(p *predictv1.PredictParam) {
		p.Fuel = arg
	}
}

func CopyRaceParam(p *predictv1.RaceParam, opts ...raceParamOption) *predictv1.RaceParam {
	ret := proto.Clone(p).(*predictv1.RaceParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithRPDuration(arg time.Duration) raceParamOption {
	return func(p *predictv1.RaceParam) {
		p.Duration = durationpb.New(arg)
	}
}

func WithRPSession(arg time.Duration) raceParamOption {
	return func(p *predictv1.RaceParam) {
		p.Session = durationpb.New(arg)
	}
}

func WithRPLc(arg int32) raceParamOption {
	return func(p *predictv1.RaceParam) {
		p.Lc = arg
	}
}

func CopyCarParam(p *predictv1.CarParam, opts ...carParamOption) *predictv1.CarParam {
	ret := proto.Clone(p).(*predictv1.CarParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithCPRemainLapTime(arg time.Duration) carParamOption {
	return func(p *predictv1.CarParam) {
		p.RemainLapTime = durationpb.New(arg)
	}
}

func WithCPCurrentTrackPos(arg float32) carParamOption {
	return func(p *predictv1.CarParam) {
		p.CurrentTrackPos = arg
	}
}

func WithCPInPit(arg bool) carParamOption {
	return func(p *predictv1.CarParam) {
		p.InPit = arg
	}
}

func WithCPStintLap(arg int32) carParamOption {
	return func(p *predictv1.CarParam) {
		p.StintLap = arg
	}
}

func CopyPitParam(p *predictv1.PitParam, opts ...pitParamOption) *predictv1.PitParam {
	ret := proto.Clone(p).(*predictv1.PitParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithPPOverall(arg time.Duration) pitParamOption {
	return func(p *predictv1.PitParam) {
		p.Overall = durationpb.New(arg)
	}
}

func WithPPLane(arg time.Duration) pitParamOption {
	return func(p *predictv1.PitParam) {
		p.Lane = durationpb.New(arg)
	}
}

func CopyStintParam(p *predictv1.StintParam, opts ...stintParamOption) *predictv1.StintParam {
	ret := proto.Clone(p).(*predictv1.StintParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithSPLps(arg int32) stintParamOption {
	return func(p *predictv1.StintParam) {
		p.Lps = arg
	}
}

func WithSPAvgLaptime(arg time.Duration) stintParamOption {
	return func(p *predictv1.StintParam) {
		p.AvgLaptime = durationpb.New(arg)
	}
}

func WithSPInLaptime(arg time.Duration) stintParamOption {
	return func(p *predictv1.StintParam) {
		p.InLaptime = durationpb.New(arg)
	}
}

func WithSPOutLaptime(arg time.Duration) stintParamOption {
	return func(p *predictv1.StintParam) {
		p.OutLaptime = durationpb.New(arg)
	}
}

func CopyFuelParam(p *predictv1.FuelParam, opts ...fuelParamOption) *predictv1.FuelParam {
	ret := proto.Clone(p).(*predictv1.FuelParam)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithFPFuelPerLap(arg float32) fuelParamOption {
	return func(p *predictv1.FuelParam) {
		p.LapConsumption = arg
	}
}

func WithFPRefuelRate(arg float32) fuelParamOption {
	return func(p *predictv1.FuelParam) {
		p.RefuelRate = arg
	}
}

func WithFPTankVolume(arg float32) fuelParamOption {
	return func(p *predictv1.FuelParam) {
		p.TankVolume = arg
	}
}
