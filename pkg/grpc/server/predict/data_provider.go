package predict

import (
	"context"
	"errors"
	"slices"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	aRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/analysis/proto"
	carRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	eventRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/event"
	rsRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	trackRepo "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

type (
	predictData struct {
		analyis        *analysisv1.Analysis
		carInfo        *racestatev1.PublishDriverDataRequest
		curState       *racestatev1.PublishStateRequest
		event          *eventv1.Event
		track          *trackv1.Track
		carIdxByCarNum map[string]int32
		carNumByIdx    map[int32]string
		carNum         string
		l              *log.Logger
		myData         *driveData
		laptimeSel     predictv1.LaptimeSelector
	}
	driveData struct {
		stints    []*stintStats
		pits      []*pitStats
		stateData *racestatev1.Car
	}
)

var (
	ErrNoData       = errors.New("no data found")
	ErrNeedMoreData = errors.New("not enough data to create PredictParam")
)

//nolint:whitespace,funlen // readability
func GetPredictParam(
	ctx context.Context,
	pool *pgxpool.Pool,
	eventId int,
	sessionTime time.Duration,
	carNum string,
	laptimeSel predictv1.LaptimeSelector,
) (*predictv1.PredictParam, error) {
	var err error
	var event *eventv1.Event
	var track *trackv1.Track
	var analysis *analysisv1.Analysis
	var carInfo *racestatev1.PublishDriverDataRequest

	if event, err = eventRepo.LoadById(ctx, pool, eventId); err != nil {
		return nil, err
	}
	if track, err = trackRepo.LoadById(ctx, pool, int(event.TrackId)); err != nil {
		return nil, err
	}

	if analysis, err = aRepo.LoadByEventId(ctx, pool, eventId); err != nil {
		return nil, err
	}
	if carInfo, err = carRepo.LoadLatest(ctx, pool, eventId); err != nil {
		return nil, err
	}

	var states *util.RangeContainer[racestatev1.PublishStateRequest]
	if states, err = rsRepo.LoadRangeBySessionTime(
		ctx,
		pool,
		eventId,
		sessionTime.Seconds(),
		1); err != nil || len(states.Data) == 0 {
		return nil, err
	}
	pd := &predictData{
		analyis:    analysis,
		carInfo:    carInfo,
		curState:   states.Data[0],
		carNum:     carNum,
		l:          log.Default().Named("predict"),
		event:      event,
		track:      track,
		laptimeSel: laptimeSel,
	}

	//nolint:gocritic // false positive
	if err = pd.init(); err != nil {
		return nil, err
	}
	var ret *predictv1.PredictParam
	if ret, err = pd.PredictParam(); err != nil {
		return nil, err
	}
	return ret, nil
}

//nolint:whitespace // readability
func GetLivePredictParam(
	epd *utils.EventProcessingData,
	carNum string,
	laptimeSel predictv1.LaptimeSelector,
) (*predictv1.PredictParam, error) {
	pd := &predictData{
		analyis:    epd.LastAnalysisData,
		carInfo:    epd.LastDriverData,
		curState:   epd.LastRaceState,
		carNum:     carNum,
		l:          log.Default().Named("predict"),
		event:      epd.Event,
		track:      epd.Track,
		laptimeSel: laptimeSel,
	}

	if err := pd.init(); err != nil {
		return nil, err
	}

	if ret, err := pd.PredictParam(); err != nil {
		return nil, err
	} else {
		return ret, nil
	}
}

//nolint:lll,funlen,gocritic // readability,wip
func (pd *predictData) PredictParam() (*predictv1.PredictParam, error) {
	// the following data is best guess since we don't have enough data yet
	raceParam := pd.raceParam()

	stintParam := predictv1.StintParam{
		AvgLaptime: pd.avgLaptime(),
		Lps:        pd.lps(),
	}
	pitParam := predictv1.PitParam{
		Overall: pd.pitTime(),
		Lane:    pd.toDur(pd.calcPitLaneTime()),
	}
	carParam := predictv1.CarParam{
		CurrentTrackPos: pd.myData.stateData.TrackPos,
		InPit:           pd.myData.stateData.State == racestatev1.CarState_CAR_STATE_PIT,
		StintLap:        int32(pd.myData.stateData.StintLap),
		RemainLapTime:   pd.toDur((1 - pd.myData.stateData.TrackPos) * float32(stintParam.AvgLaptime.Seconds)),
	}
	return &predictv1.PredictParam{
		Race:  raceParam,
		Stint: &stintParam,
		Pit:   &pitParam,
		Car:   &carParam,
	}, nil
}

func (pd *predictData) avgLaptime() *durationpb.Duration {
	// if we have data, return the last laptime that was not an outlap or inlap
	lastFullLap := func() *durationpb.Duration {
		myLapsIdx := slices.IndexFunc(pd.analyis.CarLaps,
			func(c *analysisv1.CarLaps) bool { return c.CarNum == pd.carNum })
		if myLapsIdx > 0 {
			laps := pd.analyis.CarLaps[myLapsIdx].Laps
			lapsIdx := slices.IndexFunc(laps, func(l *analysisv1.Lap) bool {
				return pd.myData.stateData.Lc == l.LapNo
			})
			if lapsIdx > 0 {
				for i := lapsIdx; i >= 0; i-- {
					if laps[i].LapInfo == nil {
						return pd.toDur(laps[i].LapTime)
					}
				}
			}
		}
		return durationpb.New(time.Minute)
	}
	curStintAvg := func() *durationpb.Duration {
		if len(pd.myData.stints) > 0 {
			return pd.toDur(pd.myData.stints[len(pd.myData.stints)-1].rAvg)
		} else {
			return lastFullLap()
		}
	}
	switch pd.laptimeSel {
	case predictv1.LaptimeSelector_LAPTIME_SELECTOR_LAST:
		return lastFullLap()
	case predictv1.LaptimeSelector_LAPTIME_SELECTOR_CURRENT_STINT_AVG:
		if len(pd.myData.stints) > 0 {
			return pd.toDur(pd.myData.stints[len(pd.myData.stints)-1].rAvg)
		}
	case
		predictv1.LaptimeSelector_LAPTIME_SELECTOR_PREVIOUS_STINT_AVG,
		predictv1.LaptimeSelector_LAPTIME_SELECTOR_UNSPECIFIED:
		if len(pd.myData.stints) > 1 {
			return pd.toDur(pd.myData.stints[len(pd.myData.stints)-2].rAvg)
		} else {
			return curStintAvg()
		}
	}
	// if no data can be computed, return a default value of 1 minute
	return durationpb.New(time.Minute)
}

func (pd *predictData) lps() int32 {
	if len(pd.myData.stints) > 1 {
		return int32(pd.myData.stints[len(pd.myData.stints)-2].numLaps)
	}
	if len(pd.myData.stints) > 0 {
		return int32(pd.myData.stints[len(pd.myData.stints)-1].numLaps)
	}
	return 20
}

func (pd *predictData) raceParam() *predictv1.RaceParam {
	return &predictv1.RaceParam{
		Duration: pd.toDur(pd.curState.Session.TimeRemain),
		Lc:       pd.myData.stateData.Lc,
		Session:  pd.toDur(pd.curState.Session.SessionTime),
	}
}

func (pd *predictData) calcPitLaneTime() float32 {
	if pd.track.PitInfo == nil {
		return 0
	}
	return pd.track.PitInfo.LaneLength / (pd.event.PitSpeed / 3.6)
}

func (pd *predictData) pitTime() *durationpb.Duration {
	if len(pd.myData.pits) > 0 {
		return pd.toDur(pd.myData.pits[len(pd.myData.pits)-1].timeUsed)
	}
	return durationpb.New(time.Minute)
}

func (pd *predictData) toDur(sec float32) *durationpb.Duration {
	return durationpb.New(time.Duration(sec * float32(time.Second)))
}

func (pd *predictData) init() error {
	pd.carIdxByCarNum = make(map[string]int32)
	pd.carNumByIdx = make(map[int32]string)
	for _, c := range pd.carInfo.Entries {
		pd.carIdxByCarNum[c.Car.CarNumber] = int32(c.Car.CarIdx)
		pd.carNumByIdx[int32(c.Car.CarIdx)] = c.Car.CarNumber
	}
	if len(pd.curState.Cars) == 0 {
		pd.l.Error("no car data found")
		return ErrNoData
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
	if myStintIdx < 0 {
		return pits
	}
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
			}
			continue
		}
		if i == len(laps)-1 {
			if l.LapInfo != nil && l.LapInfo.Mode == commonv1.LapMode_LAP_MODE_INLAP {
				sum += l.LapInfo.Time
				calcLaps++
			}
			continue
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
