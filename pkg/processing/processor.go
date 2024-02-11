package processing

import (
	"encoding/json"
	"sort"

	"github.com/wI2L/jsondiff"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
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
	CombinedData     model.AnalysisDataWithPatches
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
	p.carProcessor.ProcessAnalysisData(analysisData)
	p.raceProcessor.ProcessAnalysisData(analysisData)
	p.allInfoMsgs = analysisData.InfoMsgs
}

// ProcessState processes the given stateMsg
//
//nolint:funlen // ok by design
func (p *Processor) ProcessState(stateMsg *model.StateData) {
	p.CombinedData = model.AnalysisDataWithPatches{}

	// create copies to compute the patches later
	preCurrentData := jsonClone(p.CurrentData)
	preRaceGraph := jsonClone(p.raceProcessor.RaceGraph)

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

	patch, err := jsondiff.Compare(preCurrentData, p.CurrentData,
		jsondiff.Factorize(), jsondiff.Rationalize(),
		jsondiff.Ignores("/cars", "/session", "/raceGraph", "/raceOrder"))
	if err != nil {
		log.Debug("Error comparing analysisData", log.ErrorField(err))
		return
	}
	p.CombinedData = model.AnalysisDataWithPatches{
		Cars:             p.CurrentData.Cars,
		Session:          p.CurrentData.Session,
		RaceOrder:        p.CurrentData.RaceOrder,
		Patches:          jsondiff.Patch{},
		RaceGraphPatches: []model.AnalysisRaceGraphPatch{},
	}
	if patch != nil {
		p.CombinedData.Patches = patch
	}

	for _, c := range p.getSortedClasses() {
		cPatch, _ := jsondiff.Compare(preRaceGraph[c], p.raceProcessor.RaceGraph[c],
			jsondiff.Factorize(), jsondiff.Rationalize())

		if cPatch != nil {
			p.CombinedData.RaceGraphPatches = append(p.CombinedData.RaceGraphPatches,
				model.AnalysisRaceGraphPatch{
					CarClass: c,
					Patches:  cPatch,
				})
		}
	}
}

func (p *Processor) ProcessCarData(carMsg *model.CarData) {
	p.carProcessor.ProcessCarPayload(&carMsg.Payload)
}

func (p *Processor) GetData() *model.AnalysisData {
	return p.CurrentData
}

func (p *Processor) GetCombinedPatchData() *model.AnalysisDataWithPatches {
	return &p.CombinedData
}

func (p *Processor) getSortedClasses() []string {
	var classes []string
	for k := range p.raceProcessor.RaceGraph {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	return classes
}

func (p *Processor) composeAnalysisData() {
	raceOrder := p.raceProcessor.RaceOrder // to keep names shorter

	classes := sortedMapKeys(p.raceProcessor.RaceGraph)
	carNums := sortedMapKeys(p.carProcessor.ByCarNum)

	// sorted by key names
	p.CurrentData = &model.AnalysisData{
		RaceOrder:       raceOrder,
		CarLaps:         flattenByReference(p.raceProcessor.CarLaps, carNums),
		CarComputeState: flattenByReference(p.carProcessor.ComputeState, carNums),
		CarStints:       flattenByReference(p.carProcessor.StintLookup, carNums),
		CarPits:         flattenByReference(p.carProcessor.PitLookup, carNums),
		CarInfo:         flattenByReference(p.carProcessor.CarInfoLookup, carNums),
		RaceGraph:       flattenRaceGraph(p.raceProcessor.RaceGraph, classes),
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

func sortedMapKeys[E any](m map[string]E) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func jsonClone[E any](data E) E {
	b, _ := json.Marshal(data)
	var ret E
	_ = json.Unmarshal(b, &ret)
	return ret
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
