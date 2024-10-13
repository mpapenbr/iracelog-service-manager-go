package race

import (
	"fmt"
	"slices"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
)

type RaceProcessor struct {
	// carNum
	RaceOrder  []string
	CarLaps    map[string]*analysisv1.CarLaps     // key carNum
	RaceGraph  map[string][]*analysisv1.RaceGraph // key: "overall", carClass names
	ReplayInfo eventv1.ReplayInfo
	// internal
	carProcessor *car.CarProcessor

	raceSessions         []uint32
	raceStartMarkerFound bool
	timeInfos            map[string]*racestatev1.TimeInfo // key carNum
}

type RaceProcessorOption func(rp *RaceProcessor)

func WithCarProcessor(cp *car.CarProcessor) RaceProcessorOption {
	return func(rp *RaceProcessor) {
		rp.carProcessor = cp
	}
}

func WithRaceSessions(raceSessions []uint32) RaceProcessorOption {
	return func(rp *RaceProcessor) {
		rp.raceSessions = raceSessions
	}
}

func NewRaceProcessor(opts ...RaceProcessorOption) *RaceProcessor {
	ret := &RaceProcessor{
		RaceOrder:            make([]string, 0),
		RaceGraph:            make(map[string][]*analysisv1.RaceGraph),
		CarLaps:              make(map[string]*analysisv1.CarLaps),
		ReplayInfo:           eventv1.ReplayInfo{},
		raceStartMarkerFound: false,
		timeInfos:            make(map[string]*racestatev1.TimeInfo),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// processes the given state message.
// This message must be already processed by the CarProcessor
func (p *RaceProcessor) ProcessStatePayload(payload *racestatev1.PublishStateRequest) {
	sessionTime := payload.Session.SessionTime

	if !p.checkValidData(payload) {
		return
	}

	// race order
	p.RaceOrder = make([]string, 0)
	for i := range payload.Cars {
		carNum := p.carProcessor.NumByIdx[uint32(payload.Cars[i].CarIdx)]
		p.RaceOrder = append(p.RaceOrder, carNum)
	}
	// car laps
	p.processCarLaps(payload)
	// race graph
	p.processOverallRaceGraph(payload)
	// race graph for each car class
	for _, carClass := range p.carProcessor.CarClasses {
		p.processClassRaceGraph(payload, carClass)
	}

	// replay info
	p.processReplayInfo(payload, sessionTime)
}

//nolint:whitespace // can't make the linters happy
func (p *RaceProcessor) processReplayInfo(
	payload *racestatev1.PublishStateRequest,
	sessionTime float32,
) {
	if !slices.Contains(p.raceSessions, payload.Session.SessionNum) {
		return
	}

	p.ReplayInfo.MaxSessionTime = sessionTime
	if !p.raceStartMarkerFound {
		foundIndex := slices.IndexFunc(payload.Messages,
			func(m *racestatev1.Message) bool {
				return m.Msg == "Race start" &&
					m.Type == racestatev1.MessageType_MESSAGE_TYPE_TIMING &&
					m.SubType == racestatev1.MessageSubType_MESSAGE_SUB_TYPE_RACE_CONTROL
			})
		if foundIndex != -1 {
			p.ReplayInfo.MinTimestamp = timestamppb.New(payload.Timestamp.AsTime())
			p.ReplayInfo.MinSessionTime = sessionTime
			p.raceStartMarkerFound = true
		} else if p.ReplayInfo.MinTimestamp == nil {
			p.ReplayInfo.MinTimestamp = timestamppb.New(payload.Timestamp.AsTime())
			p.ReplayInfo.MinSessionTime = sessionTime
		}
	}
}

//nolint:whitespace // can't make the linters happy
func (p *RaceProcessor) processOverallRaceGraph(
	payload *racestatev1.PublishStateRequest,
) {
	leaderEntry := slices.IndexFunc(payload.Cars,
		func(item *racestatev1.Car) bool {
			return item.Pos == 1
		})
	if leaderEntry == -1 {
		return
	}

	// this will be the new entry for that lap
	raceGraphLapEntry := analysisv1.RaceGraph{
		LapNo:    payload.Cars[leaderEntry].Lc,
		CarClass: "overall",
		Gaps:     make([]*analysisv1.GapInfo, 0),
	}
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		gapInfo := &analysisv1.GapInfo{
			CarNum: p.carProcessor.NumByIdx[uint32(carMsgEntry.CarIdx)],
			LapNo:  carMsgEntry.Lc,
			Pos:    carMsgEntry.Pos,
			Gap:    carMsgEntry.Gap,
			Pic:    carMsgEntry.Pic,
		}

		raceGraphLapEntry.Gaps = append(raceGraphLapEntry.Gaps, gapInfo)
	}
	slices.SortStableFunc(raceGraphLapEntry.Gaps, func(a, b *analysisv1.GapInfo) int {
		return int(a.Pos - b.Pos)
	})
	// remove existing entry from slice
	lapEntry := slices.IndexFunc(p.RaceGraph["overall"],
		func(item *analysisv1.RaceGraph) bool {
			return item.LapNo == raceGraphLapEntry.LapNo
		})
	if lapEntry != -1 {
		p.RaceGraph["overall"][lapEntry] = &raceGraphLapEntry
	} else {
		p.RaceGraph["overall"] = append(p.RaceGraph["overall"], &raceGraphLapEntry)
	}
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *RaceProcessor) processClassRaceGraph(
	payload *racestatev1.PublishStateRequest,
	carClass *carv1.CarClass,
) {
	carClassEntries := p.filterByCarClass(payload, carClass)
	if len(carClassEntries) == 0 {
		return
	}

	leaderEntry := slices.IndexFunc(carClassEntries,
		func(item *racestatev1.Car) bool {
			return item.Pic == 1
		})

	if leaderEntry == -1 {
		return
	}

	// this will be the new entry for that lap
	raceGraphLapEntry := analysisv1.RaceGraph{
		LapNo:    payload.Cars[leaderEntry].Lc,
		CarClass: carClass.Name,
		Gaps:     make([]*analysisv1.GapInfo, 0),
	}
	for i := range carClassEntries {
		carMsgEntry := carClassEntries[i]
		gapInfo := &analysisv1.GapInfo{
			CarNum: p.carProcessor.NumByIdx[uint32(carMsgEntry.CarIdx)],
			LapNo:  carMsgEntry.Lc,
			Pos:    carMsgEntry.Pos,
			Gap:    carMsgEntry.Gap - carClassEntries[leaderEntry].Gap,
			Pic:    carMsgEntry.Pic,
		}

		raceGraphLapEntry.Gaps = append(raceGraphLapEntry.Gaps, gapInfo)
	}
	slices.SortStableFunc(raceGraphLapEntry.Gaps, func(a, b *analysisv1.GapInfo) int {
		return int(a.Pos - b.Pos)
	})
	// remove existing entry from slice
	lapEntry := slices.IndexFunc(p.RaceGraph[carClass.Name],
		func(item *analysisv1.RaceGraph) bool {
			return item.LapNo == raceGraphLapEntry.LapNo
		})
	if lapEntry != -1 {
		p.RaceGraph[carClass.Name][lapEntry] = &raceGraphLapEntry
	} else {
		p.RaceGraph[carClass.Name] = append(p.RaceGraph[carClass.Name], &raceGraphLapEntry)
	}
}

// collects all cars for the given car class from a state payload
//
//nolint:whitespace // can't make the linters happy
func (p *RaceProcessor) filterByCarClass(
	payload *racestatev1.PublishStateRequest,
	carClass *carv1.CarClass,
) []*racestatev1.Car {
	ret := make([]*racestatev1.Car, 0)
	for i := range payload.Cars {
		carIdx := uint32(payload.Cars[i].CarIdx)
		if p.carProcessor.ByCarIdx[carIdx].Car.CarClassId == int32(carClass.Id) {
			ret = append(ret, payload.Cars[i])
		}
	}
	return ret
}

func (p *RaceProcessor) processCarLaps(payload *racestatev1.PublishStateRequest) {
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		carNum := p.carProcessor.NumByIdx[uint32(carMsgEntry.CarIdx)]
		lap := carMsgEntry.Lc
		laptime := carMsgEntry.Last
		if lap < 1 {
			continue // we are not interested in laps >=1
		}
		carEntry, ok := p.CarLaps[carNum]
		if !ok {
			carEntry = &analysisv1.CarLaps{
				CarNum: carNum,
				Laps:   make([]*analysisv1.Lap, 0),
			}
		}
		if carMsgEntry.TimeInfo != nil {
			p.timeInfos[carNum] = carMsgEntry.TimeInfo
		}
		if idx := slices.IndexFunc(carEntry.Laps,
			func(item *analysisv1.Lap) bool { return item.LapNo == lap }); idx != -1 {
			// lap may be updated, so add replace it
			carEntry.Laps[idx] = &analysisv1.Lap{
				LapNo:   lap,
				LapTime: laptime.Time,
				LapInfo: carEntry.Laps[idx].LapInfo,
			}
		} else {
			lapInfo := p.getLapInfo(carMsgEntry)
			carEntry.Laps = append(carEntry.Laps, &analysisv1.Lap{
				LapNo:   lap,
				LapTime: laptime.Time,
				LapInfo: lapInfo,
			})
		}
		p.CarLaps[carNum] = carEntry
	}
}

func (p *RaceProcessor) getLapInfo(carMsgEntry *racestatev1.Car) *analysisv1.LapInfo {
	carNum := p.carProcessor.NumByIdx[uint32(carMsgEntry.CarIdx)]
	ti, ok := p.timeInfos[carNum]
	if !ok {
		return nil
	}
	if ti.LapNo != carMsgEntry.Lc {
		return nil
	}
	// delete entry from map
	delete(p.timeInfos, carNum)

	return &analysisv1.LapInfo{
		Mode: ti.LapMode,
		Time: ti.Time,
	}
}

func (p *RaceProcessor) checkValidData(payload *racestatev1.PublishStateRequest) bool {
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		carIdx := carMsgEntry.CarIdx
		if carIdx == -1 {
			return false
		}
		carNum := p.carProcessor.NumByIdx[uint32(carIdx)]
		if carNum == "" {
			// silent continue.
			fmt.Printf("CarIdx %d not found\n", carIdx)
			return false
		}
	}
	return true
}
