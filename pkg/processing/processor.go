package processing

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/race"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

type Processor struct {
	Manifests     *model.Manifests
	CurrentData   *model.AnalysisData
	carProcessor  *car.CarProcessor
	raceProcessor *race.RaceProcessor
}
type ProcessorOption func(proc *Processor)

// entry point for ism when registering a new event.
// carProcessor and raceProcessor are created here
func WithManifests(manifests *model.Manifests) ProcessorOption {
	return func(proc *Processor) {
		proc.Manifests = manifests
		pe := util.NewPayloadExtractor(manifests)
		proc.carProcessor = car.NewCarProcessor(car.WithManifests(manifests))
		proc.raceProcessor = race.NewRaceProcessor(
			race.WithCarProcessor(proc.carProcessor),
			race.WithPayloadExtractor(pe),
		)
	}
}

func WithCurrentData(currentData *model.AnalysisData) ProcessorOption {
	return func(proc *Processor) {
		proc.CurrentData = currentData
	}
}

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

// ProcessState processes the given stateMsg
func (p *Processor) ProcessState(stateMsg *model.StateData) {
	p.carProcessor.ProcessStatePayload(&stateMsg.Payload)
	p.raceProcessor.ProcessStatePayload(&stateMsg.Payload)
	p.composeAnalysisData()
}

func (p *Processor) ProcessCarData(carMsg *model.CarData) {
	p.carProcessor.ProcessCarPayload(&carMsg.Payload)
}

func (p *Processor) GetData() *model.AnalysisData {
	return p.CurrentData
}

func (p *Processor) composeAnalysisData() {
	raceOrder := p.raceProcessor.RaceOrder // to keep names shorter
	p.CurrentData = &model.AnalysisData{
		RaceOrder:       raceOrder,
		CarLaps:         flattenByReference(p.raceProcessor.CarLaps, raceOrder),
		CarComputeState: flattenByReference(p.carProcessor.ComputeState, raceOrder),
		CarStints:       flattenByReference(p.carProcessor.StintLookup, raceOrder),
		CarPits:         flattenByReference(p.carProcessor.PitLookup, raceOrder),
		RaceGraph:       p.CurrentData.RaceGraph,
	}
}

func flattenByReference[E any](data map[string]E, sortReference []string) []E {
	arr := make([]E, 0, len(data))
	for _, k := range sortReference {
		if v, ok := data[k]; ok {
			arr = append(arr, v)
		}
	}
	return arr
}
