package analysis

import (
	"context"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/processing/race"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/state"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

type (
	computeRace struct {
		ctx       context.Context
		eventData *eventv1.Event
		q         repository.Querier
		p         *processing.Processor
	}
	dataProviderImpl struct {
		states     *commonFetcher[racestatev1.PublishStateRequest]
		driverData *commonFetcher[racestatev1.PublishDriverDataRequest]
	}
	myStateSelector struct {
		event *eventv1.Event
	}
)

var (
	_ ReplayDataProvider  = (*dataProviderImpl)(nil)
	_ state.StatesRequest = (*myStateSelector)(nil)
)

//nolint:whitespace // editor/linter issue
func RecomputeAnalysis(
	ctx context.Context,
	q repository.Querier,
	eventData *eventv1.Event,
) (*analysisv1.Analysis, error) {
	work := &computeRace{
		eventData: eventData,
		ctx:       ctx,
		q:         q,
	}
	return work.recomputeEvent()
}

func (c *computeRace) recomputeEvent() (ret *analysisv1.Analysis, err error) {
	raceSessions := utils.CollectRaceSessions(c.eventData)
	carProc := car.NewCarProcessor(car.WithRaceSessions(raceSessions))

	raceProc := race.NewRaceProcessor(
		race.WithCarProcessor(carProc),
		race.WithRaceSessions(raceSessions))

	c.p = processing.NewProcessor(
		processing.WithCarProcessor(carProc),
		processing.WithRaceProcessor(raceProc))
	var dp ReplayDataProvider
	if dp, err = c.newDataProviderImpl(); err != nil {
		return nil, err
	}

	r := NewReplayTask(dp,
		WithDriverDataCallback(func(data *racestatev1.PublishDriverDataRequest) {
			c.p.ProcessCarData(data)
		}),
		WithStateCallback(func(data *racestatev1.PublishStateRequest) {
			c.p.ProcessState(data)
		}))
	if err := r.Replay(); err != nil {
		return nil, err
	}

	ret = c.p.ComposeAnalysisData()
	return ret, nil
}

func (s *myStateSelector) GetEvent() *commonv1.EventSelector {
	return &commonv1.EventSelector{
		Arg: &commonv1.EventSelector_Id{Id: int32(s.event.Id)},
	}
}

func (s *myStateSelector) GetStart() *commonv1.StartSelector {
	return &commonv1.StartSelector{
		Arg: &commonv1.StartSelector_Id{Id: 0},
	}
}

func (s *myStateSelector) GetNum() int32 {
	return 0
}

func (c *computeRace) newDataProviderImpl() (*dataProviderImpl, error) {
	dContainer, err := state.CreateDriverDataContainer(
		c.ctx,
		c.q,
		&myStateSelector{event: c.eventData},
	)
	if err != nil {
		return nil, err
	}
	dRc, err := dContainer.InitialRequest()
	if err != nil {
		return nil, err
	}
	df := &commonFetcher[racestatev1.PublishDriverDataRequest]{
		buffer: dRc.Data,
		loader: func() ([]*racestatev1.PublishDriverDataRequest, error) {
			data, fetchErr := dContainer.NextRequest()
			return data.Data, fetchErr
		},
	}
	sContainer, err := state.CreateRacestatesContainer(
		c.ctx,
		c.q,
		&myStateSelector{event: c.eventData},
	)
	if err != nil {
		return nil, err
	}
	sRc, err := sContainer.InitialRequest()
	if err != nil {
		return nil, err
	}
	sf := &commonFetcher[racestatev1.PublishStateRequest]{
		buffer: sRc.Data,
		loader: func() ([]*racestatev1.PublishStateRequest, error) {
			data, fetchErr := sContainer.NextRequest()
			return data.Data, fetchErr
		},
	}
	return &dataProviderImpl{
		states:     sf,
		driverData: df,
	}, nil
}

func (dp *dataProviderImpl) NextDriverData() *racestatev1.PublishDriverDataRequest {
	return dp.driverData.next()
}

func (dp *dataProviderImpl) NextStateData() *racestatev1.PublishStateRequest {
	return dp.states.next()
}
