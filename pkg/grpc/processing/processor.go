package processing

import (
	"sort"

	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/race"
)

type Processor struct {
	carProcessor   *car.CarProcessor
	raceProcessor  *race.RaceProcessor
	analysisChan   chan *analysisv1.Analysis
	racestateChan  chan *racestatev1.PublishStateRequest
	driverDataChan chan *racestatev1.PublishDriverDataRequest
	speedmapChan   chan *racestatev1.PublishSpeedmapRequest
}
type ProcessorOption func(proc *Processor)

func WithCarProcessor(carProcessor *car.CarProcessor) ProcessorOption {
	return func(proc *Processor) {
		proc.carProcessor = carProcessor
		proc.raceProcessor = race.NewRaceProcessor(race.WithCarProcessor(carProcessor))
	}
}

//nolint:whitespace // can't make both editor and linter happy
func WithPublishChannels(
	analysisChan chan *analysisv1.Analysis,
	racestateChan chan *racestatev1.PublishStateRequest,
	driverDataChan chan *racestatev1.PublishDriverDataRequest,
	speedmapChan chan *racestatev1.PublishSpeedmapRequest,
) ProcessorOption {
	return func(proc *Processor) {
		proc.analysisChan = analysisChan
		proc.racestateChan = racestateChan
		proc.driverDataChan = driverDataChan
		proc.speedmapChan = speedmapChan
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
	if p.analysisChan != nil {
		p.analysisChan <- p.composeAnalysisData()
	}
	if p.racestateChan != nil {
		p.racestateChan <- payload
	}
}

func (p *Processor) ProcessCarData(payload *racestatev1.PublishDriverDataRequest) {
	p.carProcessor.ProcessCarPayload(payload)
	if p.driverDataChan != nil {
		p.driverDataChan <- payload
	}
	if p.analysisChan != nil {
		p.analysisChan <- p.composeAnalysisData()
	}
}

// actually, there is no real processing here, just forwarding to broadcaster
func (p *Processor) ProcessSpeedmap(payload *racestatev1.PublishSpeedmapRequest) {
	if p.speedmapChan != nil {
		p.speedmapChan <- payload
	}
}

func (p *Processor) composeAnalysisData() *analysisv1.Analysis {
	raceOrder := p.raceProcessor.RaceOrder // to keep names shorter

	classes := sortedMapKeys(p.raceProcessor.RaceGraph)
	carNums := sortedMapKeys(p.carProcessor.ByCarNum)

	// sorted by key names
	ret := &analysisv1.Analysis{
		RaceOrder:        raceOrder,
		CarLaps:          flattenByReference(p.raceProcessor.CarLaps, carNums),
		CarComputeStates: flattenByReference(p.carProcessor.ComputeState, carNums),
		CarStints:        flattenByReference(p.carProcessor.StintLookup, carNums),
		CarPits:          flattenByReference(p.carProcessor.PitLookup, carNums),
		CarInfos:         flattenByReference(p.carProcessor.CarInfoLookup, carNums),
		RaceGraph:        flattenRaceGraph(p.raceProcessor.RaceGraph, classes),
	}
	return ret
}

func sortedMapKeys[E any](m map[string]E) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func flattenRaceGraph[E any](data map[string][]E, sortReference []string) []E {
	arr := make([]E, 0, len(data))
	for _, k := range sortReference {
		arr = append(arr, data[k]...)
	}
	return arr
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
