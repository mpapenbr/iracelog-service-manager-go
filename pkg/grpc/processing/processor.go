package processing

import (
	"sort"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/race"
)

// Processors is responsible for for processing incoming publish-messages.
// It knows which components to call and which channels to publish to.
type Processor struct {
	carProcessor     *car.CarProcessor
	raceProcessor    *race.RaceProcessor
	analysisChan     chan *analysisv1.Analysis
	racestateChan    chan *racestatev1.PublishStateRequest
	driverDataChan   chan *racestatev1.PublishDriverDataRequest
	speedmapChan     chan *racestatev1.PublishSpeedmapRequest
	replayInfoChan   chan *eventv1.ReplayInfo
	snapshotChan     chan *analysisv1.SnapshotData
	lastSnapshotTime time.Time
	lastSessionData  *racestatev1.Session // used for environment data (weather, etc)
}
type ProcessorOption func(proc *Processor)

func WithCarProcessor(carProcessor *car.CarProcessor) ProcessorOption {
	return func(proc *Processor) {
		proc.carProcessor = carProcessor
	}
}

func WithRaceProcessor(raceProcessor *race.RaceProcessor) ProcessorOption {
	return func(proc *Processor) {
		proc.raceProcessor = raceProcessor
	}
}

//nolint:whitespace // can't make both editor and linter happy
func WithPublishChannels(
	analysisChan chan *analysisv1.Analysis,
	racestateChan chan *racestatev1.PublishStateRequest,
	driverDataChan chan *racestatev1.PublishDriverDataRequest,
	speedmapChan chan *racestatev1.PublishSpeedmapRequest,
	replayInfoChan chan *eventv1.ReplayInfo,
	snapshotChan chan *analysisv1.SnapshotData,
) ProcessorOption {
	return func(proc *Processor) {
		proc.analysisChan = analysisChan
		proc.racestateChan = racestateChan
		proc.driverDataChan = driverDataChan
		proc.speedmapChan = speedmapChan
		proc.replayInfoChan = replayInfoChan
		proc.snapshotChan = snapshotChan
	}
}

func NewProcessor(opts ...ProcessorOption) *Processor {
	ret := &Processor{
		lastSnapshotTime: time.Time{},
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (p *Processor) ProcessState(payload *racestatev1.PublishStateRequest) {
	p.carProcessor.ProcessStatePayload(payload)
	p.raceProcessor.ProcessStatePayload(payload)
	if p.analysisChan != nil {
		p.analysisChan <- p.ComposeAnalysisData()
	}
	if p.racestateChan != nil {
		p.racestateChan <- payload
	}
	if p.replayInfoChan != nil {
		p.replayInfoChan <- p.composeReplayInfo()
	}
	//nolint:errcheck // ignore error
	p.lastSessionData = proto.Clone(payload.Session).(*racestatev1.Session)
}

func (p *Processor) ProcessCarData(payload *racestatev1.PublishDriverDataRequest) {
	p.carProcessor.ProcessCarPayload(payload)
	if p.driverDataChan != nil {
		p.driverDataChan <- payload
	}
	if p.analysisChan != nil {
		p.analysisChan <- p.ComposeAnalysisData()
	}
}

func (p *Processor) ProcessSpeedmap(payload *racestatev1.PublishSpeedmapRequest) {
	if p.speedmapChan != nil {
		p.speedmapChan <- payload
	}
	if p.snapshotChan != nil {
		condition := payload.Timestamp.AsTime().Sub(p.lastSnapshotTime) > 2*time.Minute
		if condition {
			carClassLaptimes := make(map[string]float32)
			for k, car := range payload.Speedmap.Data {
				carClassLaptimes[k] = car.Laptime
			}
			p.snapshotChan <- &analysisv1.SnapshotData{
				//nolint:errcheck // ignore error
				RecordStamp:      proto.Clone(payload.Timestamp).(*timestamppb.Timestamp),
				SessionTime:      p.lastSessionData.SessionTime,
				TimeOfDay:        p.lastSessionData.TimeOfDay,
				AirTemp:          p.lastSessionData.AirTemp,
				TrackTemp:        p.lastSessionData.TrackTemp,
				TrackWetness:     p.lastSessionData.TrackWetness,
				Precipitation:    p.lastSessionData.Precipitation,
				CarClassLaptimes: carClassLaptimes,
			}
			p.lastSnapshotTime = payload.Timestamp.AsTime()
		}
	}
}

//nolint:errcheck // ignore error
func (p *Processor) composeReplayInfo() *eventv1.ReplayInfo {
	ret := proto.Clone(p.raceProcessor.ReplayInfo.ProtoReflect().Interface())
	return ret.(*eventv1.ReplayInfo)
}

func (p *Processor) ComposeAnalysisData() *analysisv1.Analysis {
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
		CarOccupancies:   flattenByReference(p.carProcessor.CarOccupancyLookup, carNums),
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
