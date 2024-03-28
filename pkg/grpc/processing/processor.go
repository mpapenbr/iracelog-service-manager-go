package processing

import (
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/race"
)

type Processor struct {
	carProcessor  *car.CarProcessor
	raceProcessor *race.RaceProcessor
}
type ProcessorOption func(proc *Processor)

func WithCarProcessor(carProcessor *car.CarProcessor) ProcessorOption {
	return func(proc *Processor) {
		proc.carProcessor = carProcessor
		proc.raceProcessor = race.NewRaceProcessor(race.WithCarProcessor(carProcessor))
	}
}

func NewProcessor(opts ...ProcessorOption) *Processor {
	ret := &Processor{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (p *Processor) ProcessState(payload *racestatev1.PublishStateRequest) {
	p.carProcessor.ProcessStatePayload(payload)
	p.raceProcessor.ProcessStatePayload(payload)
}

func (p *Processor) ProcessCarData(payload *racestatev1.PublishDriverDataRequest) {
	p.carProcessor.ProcessCarPayload(payload)
}
