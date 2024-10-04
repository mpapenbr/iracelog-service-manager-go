package predict

import (
	"context"
	"errors"
	"slices"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/racestints"
	"github.com/samber/lo"
)

type (
	predictData struct {
		ctx            context.Context
		analyis        *analysisv1.Analysis
		carInfo        *racestatev1.PublishDriverDataRequest
		curState       *racestatev1.PublishStateRequest
		carIdxByCarNum map[string]int32
		carNumByIdx    map[int32]string
		carNum         string
		l              *log.Logger
		myData         *driveData
	}
	driveData struct {
		stints []*stintStats
		pits   []*pitStats
	}
	Prediction struct{}
)

func NewPrediction(
	analysisData *analysisv1.Analysis,
	carInfo *racestatev1.PublishDriverDataRequest,
	currentState *racestatev1.PublishStateRequest,
	carNum string,
) *Prediction {
	pd := &predictData{
		analyis:  analysisData,
		carInfo:  carInfo,
		curState: currentState,
		carNum:   carNum,
		l:        log.Default().Named("predict"),
	}
	pd.init()
	pd.calcWithLatest()
	return &Prediction{}
}

func (dd *driveData) getStintStats() []*stintStats {
	return dd.stints
}

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
		stints: pd.collectStints(pd.carNum, pd.curState.Cars[0].Lc),
		pits:   pd.collectPitstops(pd.carNum, pd.curState.Cars[0].Lc),
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
