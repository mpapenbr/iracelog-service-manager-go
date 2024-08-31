package car

import (
	"fmt"
	"slices"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

const (
	StateInit     = "INIT"
	StateRun      = "RUN"
	StatePit      = "PIT"
	StateStop     = "STOP"
	StateOut      = "OUT"
	OUT_THRESHOLD = 60.0
)

type CarProcessor struct {
	Manifests        *model.Manifests
	ByCarIdx         map[int]*model.CarEntry
	ByCarNum         map[string]*model.CarEntry
	ComputeState     map[string]model.AnalysisCarComputeState
	StintLookup      map[string]model.AnalysisCarStints
	PitLookup        map[string]model.AnalysisCarPits
	CarInfoLookup    map[string]model.AnalysisCarInfo
	NumByIdx         map[int]string
	CurrentDrivers   map[int]string
	CarClasses       []model.CarClass
	payloadExtractor *util.PayloadExtractor
}

type CarProcessorOption func(cp *CarProcessor)

func WithByCarIdx(byCarIdx map[int]*model.CarEntry) CarProcessorOption {
	return func(cp *CarProcessor) {
		cp.ByCarIdx = byCarIdx
	}
}

func WithManifests(manifests *model.Manifests) CarProcessorOption {
	return func(cp *CarProcessor) {
		cp.Manifests = manifests
		cp.payloadExtractor = util.NewPayloadExtractor(manifests)
	}
}

func NewCarProcessor(opts ...CarProcessorOption) *CarProcessor {
	cp := &CarProcessor{
		ByCarIdx:      make(map[int]*model.CarEntry),
		ByCarNum:      make(map[string]*model.CarEntry),
		ComputeState:  make(map[string]model.AnalysisCarComputeState),
		StintLookup:   make(map[string]model.AnalysisCarStints),
		PitLookup:     make(map[string]model.AnalysisCarPits),
		CarInfoLookup: make(map[string]model.AnalysisCarInfo),
		NumByIdx:      make(map[int]string),
	}
	for _, opt := range opts {
		opt(cp)
	}
	return cp
}

//nolint:lll // by design
func (p *CarProcessor) ProcessAnalysisData(analysisData *model.AnalysisData) {
	for i := range analysisData.CarComputeState {
		p.ComputeState[analysisData.CarComputeState[i].CarNum] = analysisData.CarComputeState[i]
	}
	for i := range analysisData.CarInfo {
		p.CarInfoLookup[analysisData.CarInfo[i].CarNum] = analysisData.CarInfo[i]
	}
	for i := range analysisData.CarStints {
		p.StintLookup[analysisData.CarStints[i].CarNum] = analysisData.CarStints[i]
	}
	for i := range analysisData.CarPits {
		p.PitLookup[analysisData.CarPits[i].CarNum] = analysisData.CarPits[i]
	}
}

// gets called when a car message is received via "...live.cardata" topic
func (p *CarProcessor) ProcessCarPayload(payload *model.CarPayload) {
	p.CarClasses = payload.CarClasses
	for i := range payload.Entries {
		p.ByCarIdx[payload.Entries[i].Car.CarIdx] = &payload.Entries[i]
		p.ByCarNum[payload.Entries[i].Car.CarNumber] = &payload.Entries[i]
		p.NumByIdx[payload.Entries[i].Car.CarIdx] = payload.Entries[i].Car.CarNumber
		if _, ok := p.ComputeState[payload.Entries[i].Car.CarNumber]; !ok {
			p.ComputeState[payload.Entries[i].Car.CarNumber] = model.AnalysisCarComputeState{
				CarNum:         payload.Entries[i].Car.CarNumber,
				State:          StateInit,
				OutEncountered: 0.0,
			}
		}
	}

	p.updateCarInfo(payload)
	p.CurrentDrivers = payload.CurrentDrivers
}

//nolint:gocognit,nestif // by design
func (p *CarProcessor) updateCarInfo(payload *model.CarPayload) {
	for carIdx, name := range payload.CurrentDrivers {
		item, ok := p.CarInfoLookup[p.NumByIdx[carIdx]]
		if !ok {
			item = model.AnalysisCarInfo{
				CarNum: p.NumByIdx[carIdx],
				Name:   p.ByCarIdx[carIdx].Team.Name,
				Drivers: []model.AnalysisDriverInfo{
					p.newDriverEntry(name, payload.SessionTime),
				},
			}
		} else {
			updateLeave := func(arg *model.AnalysisDriverInfo) {
				arg.SeatTime[len(arg.SeatTime)-1].LeaveCarTime = payload.SessionTime
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
					if currentDriverIdx != -1 {
						updateLeave(currentDriverEntry)
					}
					item.Drivers[driverIndex].SeatTime = append(
						item.Drivers[driverIndex].SeatTime,
						model.AnalysisSeatTime{
							EnterCarTime: payload.SessionTime,
							LeaveCarTime: payload.SessionTime,
						})
				} else {
					updateLeave(driverEntry)
				}
			}
		}
		p.CarInfoLookup[p.NumByIdx[carIdx]] = item
	}
}

//nolint:lll // by design
func (p *CarProcessor) newDriverEntry(arg string, sessionTime float64) model.AnalysisDriverInfo {
	return model.AnalysisDriverInfo{
		DriverName: arg,
		SeatTime: []model.AnalysisSeatTime{
			{EnterCarTime: sessionTime, LeaveCarTime: sessionTime},
		},
	}
}

// gets called while processing state message if car is still active
func (p *CarProcessor) markSeatTime(carNum string, sessionTime float64) {
	entry := p.CarInfoLookup[carNum]
	carIdx := p.ByCarNum[carNum].Car.CarIdx
	_, driverEntry := p.driverEntryByName(entry.Drivers, p.CurrentDrivers[carIdx])
	if driverEntry != nil {
		driverEntry.SeatTime[len(driverEntry.SeatTime)-1].LeaveCarTime = sessionTime
	}
	p.CarInfoLookup[carNum] = entry
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) driverEntryByName(data []model.AnalysisDriverInfo, name string) (
	int, *model.AnalysisDriverInfo,
) {
	driverIndex := slices.IndexFunc(
		data,
		func(item model.AnalysisDriverInfo) bool {
			return item.DriverName == name
		})
	if driverIndex == -1 {
		return -1, nil
	} else {
		return driverIndex, &data[driverIndex]
	}
}

// gets called when multiple state message are available and needs to be processed
// (mainly in tests)
func (p *CarProcessor) ProcessStatePayloads(payloads []*model.StatePayload) {
	for i := range payloads {
		p.ProcessStatePayload(payloads[i])
	}
}

// gets called when a state message is received via "...live.state" topic
func (p *CarProcessor) ProcessStatePayload(payload *model.StatePayload) {
	sessionTime, ok := p.payloadExtractor.ExtractSessionValue(
		payload.Session, "sessionTime").(float64)
	if !ok {
		return
	}
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]

		carIdx, ok := p.getIntVal(p.payloadExtractor.ExtractCarValue(carMsgEntry, "carIdx"))
		if !ok {
			fmt.Printf("Error extracting carIdx\n")
			continue
		}
		carNum := p.NumByIdx[carIdx]
		if carNum == "" {
			// silent continue.
			fmt.Printf("CarIdx %d not found\n", carIdx)
			continue
		}
		carComputeState := p.ComputeState[carNum]

		p.handleComputeState(&carComputeState, carMsgEntry, sessionTime)
		p.ComputeState[carNum] = carComputeState
	}
}

//nolint:whitespace,errcheck // can't make the linters happy
func (p *CarProcessor) handleComputeState(
	carComputeState *model.AnalysisCarComputeState,
	carMsg []interface{},
	sessionTime float64,
) {
	curCarLap, _ := p.getIntVal(p.payloadExtractor.ExtractCarValue(carMsg, "lap"))
	// we don't want data for cars that are not yet active (lap < 1)
	if curCarLap < 1 {
		return
	}
	curCarState := p.payloadExtractor.ExtractCarValue(carMsg, "state").(string)

	switch carComputeState.State {
	case StateInit:
		p.handleComputeStateInit(carComputeState, curCarLap, curCarState, sessionTime)

	case StateRun:
		p.handleComputeStateRun(carComputeState, curCarLap, curCarState, sessionTime)

	case StatePit:
		p.handleComputeStatePit(carComputeState, curCarLap, curCarState, sessionTime)

	case StateOut:
		p.handleComputeStateOut(carComputeState, curCarLap, curCarState, sessionTime)

	}
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) handleComputeStateInit(
	carComputeState *model.AnalysisCarComputeState,
	curCarLap int,
	curCarState string,
	sessionTime float64,
) {
	switch curCarState {
	case StateRun:
		stints := model.AnalysisCarStints{
			CarNum: carComputeState.CarNum,
			Current: model.AnalysisStintInfo{
				ExitTime:       sessionTime,
				LapExit:        curCarLap,
				IsCurrentStint: true,
			},
			History: []model.AnalysisStintInfo{},
		}
		p.StintLookup[carComputeState.CarNum] = stints
		carComputeState.State = StateRun
		p.markSeatTime(carComputeState.CarNum, sessionTime)

	case StatePit: // empty by design
	default:
	}
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) handleComputeStateRun(
	carComputeState *model.AnalysisCarComputeState,
	curCarLap int,
	curCarState string,
	sessionTime float64,
) {
	stint := p.StintLookup[carComputeState.CarNum]
	// these values are "precomputed" in case the stints ends or car goes to "OUT"
	stint.Current.EnterTime = sessionTime
	stint.Current.LapEnter = curCarLap
	stint.Current.NumLaps = curCarLap - stint.Current.LapExit + 1
	stint.Current.StintTime = sessionTime - stint.Current.ExitTime
	p.markSeatTime(carComputeState.CarNum, sessionTime)
	switch curCarState {
	case StateRun:
		carComputeState.OutEncountered = 0.0
	case StateOut:
		if carComputeState.OutEncountered == 0.0 {
			carComputeState.OutEncountered = sessionTime
		} else if sessionTime-carComputeState.OutEncountered > OUT_THRESHOLD {
			// stint ended
			stint.History = append(stint.History, stint.Current)
			// reset current stint
			stint.Current = model.AnalysisStintInfo{IsCurrentStint: false}
			carComputeState.State = StateOut
		}

	case StatePit:
		carComputeState.OutEncountered = 0.0
		carComputeState.State = StatePit
		stint.Current.IsCurrentStint = false
		stint.History = append(stint.History, stint.Current)
		// reset current stint data
		stint.Current = model.AnalysisStintInfo{IsCurrentStint: false}
		pits, ok := p.PitLookup[carComputeState.CarNum]
		if !ok {
			pits = model.AnalysisCarPits{
				CarNum:  carComputeState.CarNum,
				History: []model.AnalysisPitInfo{},
			}
		}
		pits.Current = model.AnalysisPitInfo{
			EnterTime:        sessionTime,
			LapEnter:         curCarLap,
			IsCurrentPitstop: true,
		}
		p.PitLookup[carComputeState.CarNum] = pits
	}
	p.StintLookup[carComputeState.CarNum] = stint
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) handleComputeStatePit(
	carComputeState *model.AnalysisCarComputeState,
	curCarLap int,
	curCarState string,
	sessionTime float64,
) {
	pits := p.PitLookup[carComputeState.CarNum]
	pits.Current.ExitTime = sessionTime
	pits.Current.LapExit = curCarLap
	pits.Current.LaneTime = sessionTime - pits.Current.EnterTime
	p.markSeatTime(carComputeState.CarNum, sessionTime)
	switch curCarState {
	case StateRun:
		carComputeState.OutEncountered = 0.0
		carComputeState.State = StateRun
		pits.Current.IsCurrentPitstop = false
		pits.History = append(pits.History, pits.Current)
		// reset current pit data
		pits.Current = model.AnalysisPitInfo{IsCurrentPitstop: false}

		// create a new stint
		stints := p.StintLookup[carComputeState.CarNum]
		stints.Current = model.AnalysisStintInfo{
			ExitTime:       sessionTime,
			LapExit:        curCarLap,
			IsCurrentStint: true,
		}
		p.StintLookup[carComputeState.CarNum] = stints
	case StateOut:
		if carComputeState.OutEncountered == 0.0 {
			carComputeState.OutEncountered = sessionTime
		} else if sessionTime-carComputeState.OutEncountered > OUT_THRESHOLD {
			carComputeState.State = StateOut
		}
	case StatePit:
		carComputeState.OutEncountered = 0.0
	}
	p.PitLookup[carComputeState.CarNum] = pits
}

//nolint:whitespace,funlen // can't make the linters happy
func (p *CarProcessor) handleComputeStateOut(
	carComputeState *model.AnalysisCarComputeState,
	curCarLap int,
	curCarState string,
	sessionTime float64,
) {
	switch curCarState {
	case StateRun:
		stints := p.StintLookup[carComputeState.CarNum]
		stints.Current = model.AnalysisStintInfo{
			ExitTime:       sessionTime,
			LapExit:        curCarLap,
			IsCurrentStint: true,
		}
		carComputeState.State = StateRun
		p.StintLookup[carComputeState.CarNum] = stints
	case StatePit:
		pits, ok := p.PitLookup[carComputeState.CarNum]
		if !ok {
			pits = model.AnalysisCarPits{
				CarNum:  carComputeState.CarNum,
				History: []model.AnalysisPitInfo{},
			}
		}
		pits.Current = model.AnalysisPitInfo{
			EnterTime:        sessionTime,
			LapEnter:         curCarLap,
			IsCurrentPitstop: true,
		}
		p.PitLookup[carComputeState.CarNum] = pits
		carComputeState.State = StatePit

	}
}

func (p *CarProcessor) getIntVal(rawVal interface{}) (int, bool) {
	switch val := rawVal.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	default:
		fmt.Printf("Error extracting int val: %v %T\n", rawVal, rawVal)
		return -1, false
	}
}
