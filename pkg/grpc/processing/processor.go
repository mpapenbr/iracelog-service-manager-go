package processing

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/race"
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
