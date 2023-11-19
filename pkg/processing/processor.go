package processing

import (
	"fmt"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/race"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

type Processor struct {
	Manifests        *model.Manifests
	CurrentData      *model.AnalysisData
	payloadExtractor *util.PayloadExtractor
	carProcessor     *car.CarProcessor
	raceProcessor    *race.RaceProcessor
	latestState      *model.StateData
	allInfoMsgs      []model.AnalysisMessage
	ReplayInfo       model.ReplayInfo
}
type ProcessorOption func(proc *Processor)

// entry point for ism when registering a new event.
// carProcessor and raceProcessor are created here
func WithManifests(manifests *model.Manifests, raceSession int) ProcessorOption {
	return func(proc *Processor) {
		proc.Manifests = manifests
		proc.payloadExtractor = util.NewPayloadExtractor(manifests)
		proc.carProcessor = car.NewCarProcessor(car.WithManifests(manifests))
		proc.raceProcessor = race.NewRaceProcessor(
			race.WithCarProcessor(proc.carProcessor),
			race.WithPayloadExtractor(proc.payloadExtractor),
			race.WithRaceSession(raceSession),
		)
	}
}

func WithCarProcessor(carProcessor *car.CarProcessor) ProcessorOption {
	return func(proc *Processor) {
		proc.carProcessor = carProcessor
		proc.raceProcessor = race.NewRaceProcessor(race.WithCarProcessor(carProcessor))
	}
}

func NewProcessor(opts ...ProcessorOption) *Processor {
	ret := &Processor{
		allInfoMsgs: make([]model.AnalysisMessage, 0),
		ReplayInfo:  model.ReplayInfo{},
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// takes analysis data and initializes the carProcessor and raceProcessor with it
// Note: this is called by webfrontend when connecting to an existing event
// The processor is already initialized with  manifests and CarPayload.
func (p *Processor) ProcessAnalysisData(analysisData *model.AnalysisData) {
	fmt.Printf("received analysisData: %+v\n", analysisData)
	p.carProcessor.ProcessAnalysisData(analysisData)
	p.raceProcessor.ProcessAnalysisData(analysisData)
	p.allInfoMsgs = analysisData.InfoMsgs
}

// ProcessState processes the given stateMsg
func (p *Processor) ProcessState(stateMsg *model.StateData) {
	p.carProcessor.ProcessStatePayload(&stateMsg.Payload)
	p.raceProcessor.ProcessStatePayload(stateMsg)
	p.latestState = stateMsg
	if len(stateMsg.Payload.Messages) > 0 {
		p.allInfoMsgs = append(p.allInfoMsgs, model.AnalysisMessage{
			Type:      model.MessageType(stateMsg.Type),
			Data:      stateMsg.Payload.Messages,
			Timestamp: stateMsg.Timestamp,
		})
	}
	p.ReplayInfo = p.raceProcessor.ReplayInfo
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
		CarInfo:         flattenByReference(p.carProcessor.CarInfoLookup, raceOrder),
		RaceGraph:       flatten(p.raceProcessor.RaceGraph),
		Cars: model.AnalysisMessage{
			Type:      model.MessageType(p.latestState.Type),
			Timestamp: p.latestState.Timestamp,
			Data:      p.latestState.Payload.Cars,
		},
		Session: model.AnalysisMessage{
			Type:      model.MessageType(p.latestState.Type),
			Timestamp: p.latestState.Timestamp,
			Data:      p.latestState.Payload.Session,
		},
		InfoMsgs: p.allInfoMsgs,
	}
}

func flatten[E any](data map[string][]E) []E {
	arr := make([]E, 0, len(data))
	for _, v := range data {
		arr = append(arr, v...)
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
