package util

import (
	"context"
	"net/http"
	"sync"
	"time"

	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/v2/iracelog/provider/v1/providerv1connect"
	"buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/v2/iracelog/racestate/v1/racestatev1connect"
	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

type ReplayDataProvider interface {
	ProvideEventData(eventID int) *providerv1.RegisterEventRequest
	NextDriverData() *racestatev1.PublishDriverDataRequest
	NextStateData() *racestatev1.PublishStateRequest
	NextSpeedmapData() *racestatev1.PublishSpeedmapRequest
}
type ReplayOption func(*ReplayTask)

func NewReplayTask(dataProvider ReplayDataProvider, opts ...ReplayOption) *ReplayTask {
	ret := &ReplayTask{dataProvider: dataProvider}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithFastForward(ff time.Duration) ReplayOption {
	return func(r *ReplayTask) {
		r.fastForward = ff
	}
}

type ReplayTask struct {
	dataProvider ReplayDataProvider

	ctx              context.Context
	cancel           context.CancelFunc
	providerService  providerv1connect.ProviderServiceClient
	raceStateService racestatev1connect.RaceStateServiceClient
	event            *eventv1.Event

	wg             sync.WaitGroup
	stateChan      chan *racestatev1.PublishStateRequest
	speedmapChan   chan *racestatev1.PublishSpeedmapRequest
	driverDataChan chan *racestatev1.PublishDriverDataRequest
	fastForward    time.Duration
	ffStopTime     time.Time // time when fast forward should stop
}

func (p *peekDriverData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekDriverData) publish() error {
	log.Debug("Sending driver data", log.Time("time", p.dataReq.Timestamp.AsTime()))
	if _, err := p.r.raceStateService.PublishDriverData(p.r.ctx, p.dataReq); err != nil {
		log.Error("Error publishing driver data", log.ErrorField(err))
		return err
	}
	return nil
}

func (p *peekStateData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekStateData) publish() error {
	log.Debug("Sending state data", log.Time("time", p.dataReq.Timestamp.AsTime()))
	if _, err := p.r.raceStateService.PublishState(p.r.ctx, p.dataReq); err != nil {
		log.Error("Error publishing state data", log.ErrorField(err))
		return err
	}
	return nil
}

// --- SpeedmapData ---
func (p *peekSpeedmapData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekSpeedmapData) publish() error {
	log.Debug("Sending speedmap data", log.Time("time", p.dataReq.Timestamp.AsTime()))
	if _, err := p.r.raceStateService.PublishSpeedmap(p.r.ctx, p.dataReq); err != nil {
		log.Error("Error publishing speedmap data", log.ErrorField(err))
		return err
	}
	return nil
}

func (r *ReplayTask) Replay(eventID int) error {
	client := connect.NewClient(
		connecthttp.NewTransport(
			http.DefaultClient,
			Addr,
			connecthttp.WithReadMaxBytes(0), // keep v1 behavior (unlimited)
			connecthttp.WithGRPC(),
		),
		tokenInterceptor(),
	)
	r.providerService = providerv1connect.NewProviderServiceClient(client)
	r.raceStateService = racestatev1connect.NewRaceStateServiceClient(client)
	r.ctx, r.cancel = context.WithCancel(context.Background())
	defer r.cancel()

	r.stateChan = make(chan *racestatev1.PublishStateRequest)
	r.speedmapChan = make(chan *racestatev1.PublishSpeedmapRequest)
	r.driverDataChan = make(chan *racestatev1.PublishDriverDataRequest)

	var err error
	registerReq := r.dataProvider.ProvideEventData(eventID)

	if r.event, err = r.registerEvent(registerReq); err != nil {
		return err
	}
	log.Debug("Event registered", log.String("key", r.event.Key))

	log.Debug("Replaying event", log.String("event", r.event.Name))

	r.wg = sync.WaitGroup{}
	r.wg.Add(4)
	go r.provideDriverData()
	go r.provideStateData()
	go r.provideSpeedmapData()
	go r.sendData()

	log.Debug("Waiting for tasks to finish")
	r.wg.Wait()
	log.Debug("About to unregister event")
	err = r.unregisterEvent()
	log.Debug("Event unregistered", log.String("key", r.event.Key))

	return err
}

//nolint:funlen,gocognit,cyclop //  by design
func (r *ReplayTask) sendData() {
	defer r.wg.Done()

	pData := make([]peek, 0, 3)
	pData = append(pData, //
		&peekStateData{
			commonStateData[racestatev1.PublishStateRequest]{
				r: r, dataChan: r.stateChan, providerType: StateData,
			},
		},
		&peekDriverData{
			commonStateData[racestatev1.PublishDriverDataRequest]{
				r: r, dataChan: r.driverDataChan, providerType: DriverData,
			},
		},
		&peekSpeedmapData{
			commonStateData[racestatev1.PublishSpeedmapRequest]{
				r: r, dataChan: r.speedmapChan, providerType: SpeedmapData,
			},
		},
	)
	for _, p := range pData {
		if !p.refill() {
			log.Debug("exhausted", log.String("provider", string(p.provider())))
		}
	}
	lastTS := time.Time{}

	var selector providerType
	var current peek
	for {
		var delta time.Duration
		var currentIdx int
		// create a max time from  (don't use time.Unix(1<<63-1), that's not what we want)
		nextTS := time.Unix(0, 0).Add(1<<63 - 1)

		for i, p := range pData {
			if p.ts().Before(nextTS) {
				nextTS = p.ts()
				selector = p.provider()
				current = pData[i]
				currentIdx = i
			}
		}
		r.computeFastForwardStop(nextTS)

		if !lastTS.IsZero() {
			wait := r.calcWaitTime(nextTS, lastTS)
			if wait > 0 {
				log.Debug("Sleeping",
					log.Time("time", nextTS),
					log.Duration("delta", delta),
					log.Duration("wait", wait),
				)
				time.Sleep(wait)
			}
		}
		lastTS = nextTS

		if err := current.publish(); err != nil {
			log.Error("Error publishing data", log.ErrorField(err))
			return
		}
		if !current.refill() {
			log.Debug("exhausted", log.String("provider", string(selector)))
			pData = append(pData[:currentIdx], pData[currentIdx+1:]...)
			if len(pData) == 0 {
				log.Debug("All providers exhausted")
				return
			}
		}
	}
}

func (r *ReplayTask) computeFastForwardStop(cur time.Time) {
	if r.fastForward > 0 && r.ffStopTime.IsZero() {
		r.ffStopTime = cur.Add(r.fastForward)
		log.Debug("Fast forward stop time set", log.Time("time", r.ffStopTime))
	}
}

func (r *ReplayTask) calcWaitTime(nextTS, lastTS time.Time) time.Duration {
	delta := nextTS.Sub(lastTS)

	// handle fast forward
	if nextTS.Before(r.ffStopTime) {
		return 0
	}
	if Speed > 0 {
		return time.Duration(int(delta.Nanoseconds()) / Speed)
	} else {
		return delta
	}
}

func (r *ReplayTask) provideDriverData() {
	defer r.wg.Done()
	i := 0
	for {
		select {
		case <-r.ctx.Done():
			log.Debug("Context done")
			return
		default:
			item := r.dataProvider.NextDriverData()
			if item == nil {
				log.Debug("No more driver data")
				close(r.driverDataChan)
				return
			}
			r.driverDataChan <- item
			i++
			log.Debug("Sent data on driverDataChen", log.Int("i", i))
		}
	}
}

func (r *ReplayTask) provideStateData() {
	defer r.wg.Done()
	i := 0
	for {
		select {
		case <-r.ctx.Done():
			log.Debug("Context done")
			return
		default:
			item := r.dataProvider.NextStateData()
			if item == nil {
				log.Debug("No more state data")
				close(r.stateChan)
				return
			}
			r.stateChan <- item
			i++
			log.Debug("Sent data on stateDataChan", log.Int("i", i))
		}
	}
}

func (r *ReplayTask) provideSpeedmapData() {
	defer r.wg.Done()
	i := 0
	for {
		select {
		case <-r.ctx.Done():
			log.Debug("Context done")
			return
		default:
			item := r.dataProvider.NextSpeedmapData()
			if item == nil {
				log.Debug("No more speedmap data")
				close(r.speedmapChan)
				return
			}
			r.speedmapChan <- item
			i++
			log.Debug("Sent data on speedmapChan", log.Int("i", i))
		}
	}
}

//nolint:whitespace // linter+editor issue
func (r *ReplayTask) registerEvent(eventReq *providerv1.RegisterEventRequest) (
	*eventv1.Event, error,
) {
	resp, err := r.providerService.RegisterEvent(r.ctx, eventReq)
	if err == nil {
		return resp.Event, nil
	}
	return nil, err
}

func (r *ReplayTask) unregisterEvent() error {
	_, err := r.providerService.UnregisterEvent(r.ctx,
		&providerv1.UnregisterEventRequest{
			EventSelector: r.buildEventSelector(),
		},
	)
	return err
}

func (r *ReplayTask) buildEventSelector() *commonv1.EventSelector {
	return &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: r.event.Key}}
}

// tokenInterceptor adds the api token to each request
func tokenInterceptor() connect.ClientInterceptor {
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			if info, ok := connect.CallInfoForClientContext(ctx); ok {
				info.RequestHeader().Set("api-token", Token)
			}
			return next(ctx, spec)
		}
	}
}
