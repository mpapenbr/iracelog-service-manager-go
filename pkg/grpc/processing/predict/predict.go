package predict

import (
	"errors"
	"slices"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/samber/lo"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/racestints"
)

type (
	predictData struct {
		analyis        *analysisv1.Analysis
		carInfo        *racestatev1.PublishDriverDataRequest
		curState       *racestatev1.PublishStateRequest
		event          *eventv1.Event
		carIdxByCarNum map[string]int32
		carNumByIdx    map[int32]string
		carNum         string
		l              *log.Logger
		myData         *driveData
	}
	driveData struct {
		stints    []*stintStats
		pits      []*pitStats
		stateData *racestatev1.Car
	}
	Prediction struct{}
)

//nolint:whitespace,gocritic,errcheck // readability,wip
func NewPrediction(
	analysisData *analysisv1.Analysis,
	carInfo *racestatev1.PublishDriverDataRequest,
	currentState *racestatev1.PublishStateRequest,
	event *eventv1.Event,
	carNum string,
) *Prediction {
	pd := &predictData{
		analyis:  analysisData,
		carInfo:  carInfo,
		curState: currentState,
		carNum:   carNum,
		l:        log.Default().Named("predict"),
		event:    event,
	}
	pd.init()

	// pd.calcWithLatest()
	pd.expertCalcWithLatest()
	return &Prediction{}
}

//nolint:lll,unused // readability,wip
func (pd *predictData) calcWithLatest() {
	simpleCalc := racestints.NewSimpleStintCalc(&racestints.SimpleCalcParams{
		RaceDur: time.Duration(pd.curState.Session.TimeRemain * float32(time.Second)),
		Lps:     pd.myData.stints[len(pd.myData.stints)-1].numLaps,
		PitTime: time.Duration(pd.myData.pits[len(pd.myData.pits)-1].timeUsed * float32(time.Second)),
		AvgLap:  time.Duration(pd.myData.stints[len(pd.myData.stints)-1].rAvg * float32(time.Second)),
	})
	res, _ := simpleCalc.Calc()

	for i, p := range res.Parts {
		pd.l.Info("part", log.Int("i", i), log.String("output", p.Output()))
	}
}

//nolint:lll,gocritic // readability,wip
func (pd *predictData) expertCalcWithLatest() {
	// lastStint is the last complete stint. So we need at least 2 stints available
	if len(pd.myData.stints) < 2 {
		pd.l.Info("not enough data for expert calc")
		return
	}
	lastStint := pd.myData.stints[len(pd.myData.stints)-2]
	expertCalc := racestints.NewExpertStintCalc(&racestints.ExpertCalcParams{
		LC: int(pd.myData.stateData.Lc),
		// TODO: needs option to be set upfront (e.g. by computing race duration for race leader)
		RaceDur: time.Duration(pd.curState.Session.TimeRemain * float32(time.Second)),
		Lps:     lastStint.numLaps,
		PitTime: time.Duration(pd.myData.pits[len(pd.myData.pits)-1].timeUsed * float32(time.Second)),
		AvgLap:  time.Duration(lastStint.rAvg * float32(time.Second)),
	}, racestints.WithEolDur(func() *racestints.EndOfLapData {
		// TODO: handle corner case when car is in pit

		// this is where the car
		now := pd.curState.Session.SessionTime - pd.event.ReplayInfo.MinSessionTime
		// TODO: handle 0.0 case
		remainLapTime := (1 - pd.myData.stateData.TrackPos) * lastStint.rAvg
		pd.l.Debug("calc eol",
			log.Float32("trackPos", pd.myData.stateData.TrackPos),
			log.String("state", pd.myData.stateData.State.String()),
			log.Duration("remainLapTime", time.Duration(remainLapTime*float32(time.Second))),
			log.Int32("Lap", pd.myData.stateData.Lap),
			log.Int32("StintLap", int32(pd.myData.stateData.StintLap)),
			log.Duration("check", time.Duration((now+pd.curState.Session.TimeRemain)*float32(time.Second))),
			log.Float32("sessionTimeNow", now),
			log.Duration("now", time.Duration(now*float32(time.Second))))
		// eol := pd.calcToEndOfLap(pd.myData)
		return &racestints.EndOfLapData{
			CarInPit:      pd.myData.stateData.State == racestatev1.CarState_CAR_STATE_PIT,
			StintLap:      int(pd.myData.stateData.StintLap),
			RemainLapTime: time.Duration(remainLapTime * float32(time.Second)),
			SessionAtEol:  time.Duration((now + remainLapTime) * float32(time.Second)),
		}
	}))
	res, _ := expertCalc.Calc()

	for i, p := range res.Parts {
		pd.l.Info("part", log.Int("i", i), log.String("output", p.Output()))
	}
}

func (pd *predictData) init() error {
	pd.carIdxByCarNum = make(map[string]int32)
	pd.carNumByIdx = make(map[int32]string)
	for _, c := range pd.carInfo.Entries {
		pd.carIdxByCarNum[c.Car.CarNumber] = int32(c.Car.CarIdx)
		pd.carNumByIdx[int32(c.Car.CarIdx)] = c.Car.CarNumber
	}
	myCarIdx := pd.carIdxByCarNum[pd.carNum]
	pd.l.Debug("leader data",
		log.Int("lc", int(pd.curState.Cars[0].Lc)),
		log.String("carNum", pd.carNumByIdx[pd.curState.Cars[0].CarIdx]),
		log.Float32("trackPos", pd.curState.Cars[0].TrackPos))
	myIdx := slices.IndexFunc(pd.curState.Cars,
		func(c *racestatev1.Car) bool { return c.CarIdx == myCarIdx })
	if myIdx < 0 {
		pd.l.Error("no data found", log.String("carNum", pd.carNum))
		return errors.New("no data found")
	}
	pd.l.Debug("myData",
		log.Int("lc", int(pd.curState.Cars[myIdx].Lc)),
		log.Float32("trackPos", pd.curState.Cars[myIdx].TrackPos))
	pd.myData = &driveData{
		stints:    pd.collectStints(pd.carNum, pd.curState.Cars[0].Lc),
		pits:      pd.collectPitstops(pd.carNum, pd.curState.Cars[0].Lc),
		stateData: pd.curState.Cars[myIdx],
	}
	return nil
}

func (pd *predictData) collectStints(carNum string, lc int32) []*stintStats {
	pd.l.Debug("collect stint data", log.String("carNum", carNum), log.Int32("lc", lc))
	myStintIdx := slices.IndexFunc(pd.analyis.CarStints,
		func(c *analysisv1.CarStint) bool { return c.CarNum == carNum })
	myLapsIdx := slices.IndexFunc(pd.analyis.CarLaps,
		func(c *analysisv1.CarLaps) bool { return c.CarNum == carNum })
	stintStats := make([]*stintStats, 0)
	// first: collect rolling avg from stint laps
	for i, cs := range pd.analyis.CarStints[myStintIdx].History {
		if cs.LapExit < lc {
			st := createStintStats(
				cs.LapExit,
				min(lc, cs.LapEnter),
				pd.analyis.CarLaps[myLapsIdx].Laps)
			if st != nil {
				pd.l.Debug("stint", log.Int("idx", i), log.Any("data", cs),
					log.Int("numLaps", st.numLaps), log.Float32("rAvg", st.rAvg))
				stintStats = append(stintStats, st)
			}
		}
	}
	// second: collect rolling avg from current stint laps
	if pd.analyis.CarStints[myStintIdx].Current.LapExit < lc {
		cs := pd.analyis.CarStints[myStintIdx].Current
		st := createStintStats(
			cs.LapExit,
			min(lc, cs.LapEnter),
			pd.analyis.CarLaps[myLapsIdx].Laps)
		if st != nil {
			pd.l.Debug("currentStint", log.Any("data", cs),
				log.Int("numLaps", st.numLaps), log.Float32("rAvg", st.rAvg))
			stintStats = append(stintStats, st)
		}
	}
	return stintStats
}

func (pd *predictData) collectPitstops(carNum string, lc int32) []*pitStats {
	pd.l.Debug("collect pitstop data", log.String("carNum", carNum), log.Int32("lc", lc))
	myStintIdx := slices.IndexFunc(pd.analyis.CarPits,
		func(c *analysisv1.CarPit) bool { return c.CarNum == carNum })
	pits := make([]*pitStats, 0)
	for _, cp := range pd.analyis.CarPits[myStintIdx].History {
		if cp.LapExit < lc {
			pd.l.Debug("currentPit", log.Any("data", cp))
			pits = append(pits, &pitStats{cp.LaneTime})
		}
	}
	return pits
}

type stintStats struct {
	numLaps int
	rAvg    float32 // rolling average lap time
}
type pitStats struct {
	timeUsed float32
}

//nolint:gocognit // readability
func createStintStats(lapFrom, lapTo int32, lapData []*analysisv1.Lap) *stintStats {
	laps := lo.Filter(lapData, func(l *analysisv1.Lap, idx int) bool {
		return l.LapNo >= lapFrom && l.LapNo <= lapTo
	})
	var sum float32 = 0.0

	calcLaps := 0
	for i, l := range laps {
		if i == 0 {
			if l.LapInfo != nil && l.LapInfo.Mode == commonv1.LapMode_LAP_MODE_OUTLAP {
				sum += l.LapInfo.Time
				calcLaps++
			} else {
				continue
			}
		}
		if i == len(laps)-1 {
			if l.LapInfo != nil && l.LapInfo.Mode == commonv1.LapMode_LAP_MODE_INLAP {
				sum += l.LapInfo.Time
				calcLaps++
			} else {
				continue
			}
		}
		sum += l.LapTime
		calcLaps++
	}
	if calcLaps == 0 {
		return nil
	}
	return &stintStats{
		numLaps: len(laps),
		rAvg:    float32(sum) / float32(calcLaps),
	}
}
