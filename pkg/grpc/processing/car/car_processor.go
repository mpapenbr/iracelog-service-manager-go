package car

import (
	"fmt"
	"slices"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	carv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/car/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
)

type CarProcessor struct {
	ByCarIdx           map[uint32]*carv1.CarEntry
	ByCarNum           map[string]*carv1.CarEntry
	ComputeState       map[string]*analysisv1.CarComputeState
	StintLookup        map[string]*analysisv1.CarStint
	PitLookup          map[string]*analysisv1.CarPit
	CarOccupancyLookup map[string]*analysisv1.CarOccupancy
	NumByIdx           map[uint32]string
	CurrentDrivers     map[uint32]string
	CarClasses         []*carv1.CarClass
	raceSessions       []uint32
}

type CarProcessorOption func(cp *CarProcessor)

const OUT_THRESHOLD = 60.0

func WithRaceSessions(raceSessions []uint32) CarProcessorOption {
	return func(cp *CarProcessor) {
		cp.raceSessions = raceSessions
	}
}

func NewCarProcessor(opts ...CarProcessorOption) *CarProcessor {
	cp := &CarProcessor{
		ByCarIdx:           make(map[uint32]*carv1.CarEntry),
		ByCarNum:           make(map[string]*carv1.CarEntry),
		ComputeState:       make(map[string]*analysisv1.CarComputeState),
		StintLookup:        make(map[string]*analysisv1.CarStint),
		PitLookup:          make(map[string]*analysisv1.CarPit),
		CarOccupancyLookup: make(map[string]*analysisv1.CarOccupancy),
		NumByIdx:           make(map[uint32]string),
	}
	for _, opt := range opts {
		opt(cp)
	}
	return cp
}

// gets called when a new car state is arrived via grpc
//
//nolint:lll // by design
func (p *CarProcessor) ProcessCarPayload(payload *racestatev1.PublishDriverDataRequest) {
	p.CarClasses = payload.CarClasses
	for i := range payload.Entries {
		p.ByCarIdx[payload.Entries[i].Car.CarIdx] = payload.Entries[i]
		p.ByCarNum[payload.Entries[i].Car.CarNumber] = payload.Entries[i]
		p.NumByIdx[payload.Entries[i].Car.CarIdx] = payload.Entries[i].Car.CarNumber
		if _, ok := p.ComputeState[payload.Entries[i].Car.CarNumber]; !ok {
			p.ComputeState[payload.Entries[i].Car.CarNumber] = &analysisv1.CarComputeState{
				CarNum:         payload.Entries[i].Car.CarNumber,
				CarState:       racestatev1.CarState_CAR_STATE_INIT,
				OutEncountered: 0.0,
			}
		}
	}
	if !slices.Contains(p.raceSessions, payload.SessionNum) {
		p.CurrentDrivers = payload.CurrentDrivers
		return
	}
	p.updateCarInfo(payload)
	p.CurrentDrivers = payload.CurrentDrivers
}

//nolint:gocognit,nestif,funlen // by design
func (p *CarProcessor) updateCarInfo(payload *racestatev1.PublishDriverDataRequest) {
	for carIdx, name := range payload.CurrentDrivers {
		item, ok := p.CarOccupancyLookup[p.NumByIdx[carIdx]]
		if !ok {
			item = &analysisv1.CarOccupancy{
				CarNum:            p.NumByIdx[carIdx],
				Name:              p.ByCarIdx[carIdx].Team.Name,
				CurrentDriverName: name,
				Drivers: []*analysisv1.Driver{
					p.newDriverEntry(name, payload.SessionTime),
				},
			}
		} else {
			updateLeave := func(arg *analysisv1.Driver) {
				arg.SeatTimes[len(arg.SeatTimes)-1].LeaveCarTime = payload.SessionTime
			}

			driverIndex, driverEntry := p.driverEntryByName(item.Drivers, name)
			currentDriverIdx, currentDriverEntry := p.driverEntryByName(
				item.Drivers, p.CurrentDrivers[carIdx])
			if driverIndex == -1 {
				item.Drivers = append(item.Drivers,
					p.newDriverEntry(name, payload.SessionTime))
				if currentDriverIdx != -1 {
					updateLeave(currentDriverEntry)
				}
			} else {
				// check if we have a driver change
				if p.CurrentDrivers[carIdx] != name {
					item.CurrentDriverName = name
					if currentDriverIdx != -1 {
						updateLeave(currentDriverEntry)
					}
					item.Drivers[driverIndex].SeatTimes = append(
						item.Drivers[driverIndex].SeatTimes,
						&analysisv1.SeatTime{
							EnterCarTime: payload.SessionTime,
							LeaveCarTime: payload.SessionTime,
						})
				} else {
					updateLeave(driverEntry)
				}
			}
		}
		p.CarOccupancyLookup[p.NumByIdx[carIdx]] = item
	}
}

//nolint:lll // by design
func (p *CarProcessor) newDriverEntry(arg string, sessionTime float32) *analysisv1.Driver {
	return &analysisv1.Driver{
		Name: arg,
		SeatTimes: []*analysisv1.SeatTime{
			{EnterCarTime: sessionTime, LeaveCarTime: sessionTime},
		},
	}
}

// gets called while processing state message if car is still active
func (p *CarProcessor) markSeatTime(carNum string, sessionTime float32) {
	entry := p.CarOccupancyLookup[carNum]
	carIdx := p.ByCarNum[carNum].Car.CarIdx
	_, driverEntry := p.driverEntryByName(entry.Drivers, p.CurrentDrivers[carIdx])
	if driverEntry != nil {
		driverEntry.SeatTimes[len(driverEntry.SeatTimes)-1].LeaveCarTime = sessionTime
	}
	p.CarOccupancyLookup[carNum] = entry
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) driverEntryByName(data []*analysisv1.Driver, name string) (
	int, *analysisv1.Driver,
) {
	driverIndex := slices.IndexFunc(
		data,
		func(item *analysisv1.Driver) bool {
			return item.Name == name
		})
	if driverIndex == -1 {
		return -1, nil
	} else {
		return driverIndex, data[driverIndex]
	}
}

// gets called when multiple state message are available and needs to be processed
// (mainly in tests)
//
//nolint:lll // by design
func (p *CarProcessor) ProcessStatePayloads(payloads []*racestatev1.PublishStateRequest) {
	for i := range payloads {
		p.ProcessStatePayload(payloads[i])
	}
}

// gets called when a state message is received via "...live.state" topic
func (p *CarProcessor) ProcessStatePayload(payload *racestatev1.PublishStateRequest) {
	sessionTime := payload.Session.SessionTime

	p.ensureCarOccupancy(payload)

	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]

		carIdx := carMsgEntry.CarIdx
		carNum := p.NumByIdx[uint32(carIdx)]
		if carNum == "" {
			// silent continue.
			fmt.Printf("CarIdx %d not found\n", carIdx)
			continue
		}
		carComputeState := p.ComputeState[carNum]

		p.handleComputeState(carComputeState, carMsgEntry, sessionTime)
		p.ComputeState[carNum] = carComputeState
	}
}

// ensures that the car occupancy lookup is present for all cars in the state message
func (p *CarProcessor) ensureCarOccupancy(payload *racestatev1.PublishStateRequest) {
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		carIdx := carMsgEntry.CarIdx
		carNum := p.NumByIdx[uint32(carIdx)]
		if _, ok := p.CarOccupancyLookup[carNum]; !ok {
			p.CarOccupancyLookup[carNum] = &analysisv1.CarOccupancy{
				CarNum:            carNum,
				Name:              p.ByCarIdx[uint32(carIdx)].Team.Name,
				CurrentDriverName: p.CurrentDrivers[uint32(carIdx)],
				Drivers: []*analysisv1.Driver{
					p.newDriverEntry(p.CurrentDrivers[uint32(carIdx)],
						payload.Session.SessionTime),
				},
			}
		}
	}
}

//nolint:whitespace,errcheck,exhaustive // can't make the linters happy
func (p *CarProcessor) handleComputeState(
	carComputeState *analysisv1.CarComputeState,
	carMsg *racestatev1.Car,
	sessionTime float32,
) {
	curCarLap := carMsg.Lap
	// we don't want data for cars that are not yet active (lap < 1)
	if curCarLap < 1 {
		return
	}
	curCarState := carMsg.State

	switch carComputeState.CarState {
	case racestatev1.CarState_CAR_STATE_INIT:
		p.handleComputeStateInit(carComputeState, curCarLap, curCarState, sessionTime)

	case racestatev1.CarState_CAR_STATE_RUN:
		p.handleComputeStateRun(carComputeState, curCarLap, curCarState, sessionTime)

	case racestatev1.CarState_CAR_STATE_PIT:
		p.handleComputeStatePit(carComputeState, curCarLap, curCarState, sessionTime)

	case racestatev1.CarState_CAR_STATE_OUT:
		p.handleComputeStateOut(carComputeState, curCarLap, curCarState, sessionTime)
	default: // empty by design
	}
}

//nolint:whitespace,funlen,exhaustive // can't make the linters happy
func (p *CarProcessor) handleComputeStateInit(
	carComputeState *analysisv1.CarComputeState,
	curCarLap int32,
	curCarState racestatev1.CarState,
	sessionTime float32,
) {
	switch curCarState {
	case racestatev1.CarState_CAR_STATE_RUN:
		stints := &analysisv1.CarStint{
			CarNum: carComputeState.CarNum,
			Current: &analysisv1.StintInfo{
				ExitTime:       sessionTime,
				LapExit:        curCarLap,
				IsCurrentStint: true,
			},
			History: []*analysisv1.StintInfo{},
		}
		p.StintLookup[carComputeState.CarNum] = stints
		carComputeState.CarState = curCarState
		p.markSeatTime(carComputeState.CarNum, sessionTime)

	case racestatev1.CarState_CAR_STATE_PIT: // empty by design
	default:
	}
}

//nolint:whitespace,funlen,exhaustive // can't make the linters happy
func (p *CarProcessor) handleComputeStateRun(
	carComputeState *analysisv1.CarComputeState,
	curCarLap int32,
	curCarState racestatev1.CarState,
	sessionTime float32,
) {
	stint := p.StintLookup[carComputeState.CarNum]
	// these values are "precomputed" in case the stints ends or car goes to "OUT"
	stint.Current.EnterTime = sessionTime
	stint.Current.LapEnter = curCarLap
	stint.Current.NumLaps = curCarLap - stint.Current.LapExit + 1
	stint.Current.StintTime = sessionTime - stint.Current.ExitTime
	p.markSeatTime(carComputeState.CarNum, sessionTime)
	switch curCarState {
	case racestatev1.CarState_CAR_STATE_RUN:

		carComputeState.OutEncountered = 0.0
	case racestatev1.CarState_CAR_STATE_OUT:

		if carComputeState.OutEncountered == 0.0 {
			carComputeState.OutEncountered = sessionTime
		} else if sessionTime-carComputeState.OutEncountered > OUT_THRESHOLD {
			// stint ended
			stint.History = append(stint.History, stint.Current)
			// reset current stint
			stint.Current = &analysisv1.StintInfo{IsCurrentStint: false}
			carComputeState.CarState = racestatev1.CarState_CAR_STATE_OUT
		}

	case racestatev1.CarState_CAR_STATE_PIT:

		carComputeState.OutEncountered = 0.0
		carComputeState.CarState = racestatev1.CarState_CAR_STATE_PIT
		stint.Current.IsCurrentStint = false
		stint.History = append(stint.History, stint.Current)
		// reset current stint data
		stint.Current = &analysisv1.StintInfo{IsCurrentStint: false}
		pits, ok := p.PitLookup[carComputeState.CarNum]
		if !ok {
			pits = &analysisv1.CarPit{
				CarNum:  carComputeState.CarNum,
				History: []*analysisv1.PitInfo{},
			}
		}
		pits.Current = &analysisv1.PitInfo{
			EnterTime:        sessionTime,
			LapEnter:         curCarLap,
			IsCurrentPitstop: true,
		}
		p.PitLookup[carComputeState.CarNum] = pits
	}
	p.StintLookup[carComputeState.CarNum] = stint
}

//nolint:whitespace,funlen,exhaustive // can't make the linters happy
func (p *CarProcessor) handleComputeStatePit(
	carComputeState *analysisv1.CarComputeState,
	curCarLap int32,
	curCarState racestatev1.CarState,
	sessionTime float32,
) {
	pits := p.PitLookup[carComputeState.CarNum]
	pits.Current.ExitTime = sessionTime
	pits.Current.LapExit = curCarLap
	pits.Current.LaneTime = sessionTime - pits.Current.EnterTime
	p.markSeatTime(carComputeState.CarNum, sessionTime)
	switch curCarState {
	case racestatev1.CarState_CAR_STATE_RUN:
		carComputeState.OutEncountered = 0.0
		carComputeState.CarState = racestatev1.CarState_CAR_STATE_RUN
		pits.Current.IsCurrentPitstop = false
		pits.History = append(pits.History, pits.Current)
		// reset current pit data
		pits.Current = &analysisv1.PitInfo{IsCurrentPitstop: false}

		// create a new stint
		stints := p.StintLookup[carComputeState.CarNum]
		stints.Current = &analysisv1.StintInfo{
			ExitTime:       sessionTime,
			LapExit:        curCarLap,
			IsCurrentStint: true,
		}
		p.StintLookup[carComputeState.CarNum] = stints
	case racestatev1.CarState_CAR_STATE_OUT:
		if carComputeState.OutEncountered == 0.0 {
			carComputeState.OutEncountered = sessionTime
		} else if sessionTime-carComputeState.OutEncountered > OUT_THRESHOLD {
			carComputeState.CarState = racestatev1.CarState_CAR_STATE_OUT
		}
	case racestatev1.CarState_CAR_STATE_PIT:
		carComputeState.OutEncountered = 0.0
	}
	p.PitLookup[carComputeState.CarNum] = pits
}

//nolint:whitespace,funlen,exhaustive // can't make the linters happy
func (p *CarProcessor) handleComputeStateOut(
	carComputeState *analysisv1.CarComputeState,
	curCarLap int32,
	curCarState racestatev1.CarState,
	sessionTime float32,
) {
	switch curCarState {
	case racestatev1.CarState_CAR_STATE_RUN:
		stints := p.StintLookup[carComputeState.CarNum]
		stints.Current = &analysisv1.StintInfo{
			ExitTime:       sessionTime,
			LapExit:        curCarLap,
			IsCurrentStint: true,
		}
		carComputeState.CarState = racestatev1.CarState_CAR_STATE_RUN
		p.StintLookup[carComputeState.CarNum] = stints
	case racestatev1.CarState_CAR_STATE_PIT:
		pits, ok := p.PitLookup[carComputeState.CarNum]
		if !ok {
			pits = &analysisv1.CarPit{
				CarNum:  carComputeState.CarNum,
				History: []*analysisv1.PitInfo{},
			}
		}
		pits.Current = &analysisv1.PitInfo{
			EnterTime:        sessionTime,
			LapEnter:         curCarLap,
			IsCurrentPitstop: true,
		}
		p.PitLookup[carComputeState.CarNum] = pits
		carComputeState.CarState = racestatev1.CarState_CAR_STATE_PIT

	}
}
