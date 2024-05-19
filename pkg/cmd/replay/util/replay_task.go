package util

import (
	"context"
	"net/http"
	"sync"
	"time"

	"buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/provider/v1/providerv1connect"
	"buf.build/gen/go/mpapenbr/testrepo/connectrpc/go/testrepo/racestate/v1/racestatev1connect"
	commonv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	providerv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/provider/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

type ReplayDataProvider interface {
	ProvideEventData(eventId int) *providerv1.RegisterEventRequest
	NextDriverData() *racestatev1.PublishDriverDataRequest
	NextStateData() *racestatev1.PublishStateRequest
	NextSpeedmapData() *racestatev1.PublishSpeedmapRequest
}

func NewReplayTask(dataProvider ReplayDataProvider) *ReplayTask {
	return &ReplayTask{dataProvider: dataProvider}
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
}

func (p *peekDriverData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekDriverData) publish() error {
	log.Debug("Sending driver data", log.Time("time", p.dataReq.Timestamp.AsTime()))
	req := connect.NewRequest[racestatev1.PublishDriverDataRequest](p.dataReq)
	withToken(req.Header())
	if _, err := p.r.raceStateService.PublishDriverData(p.r.ctx, req); err != nil {
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
	req := connect.NewRequest[racestatev1.PublishStateRequest](p.dataReq)
	withToken(req.Header())
	if _, err := p.r.raceStateService.PublishState(p.r.ctx, req); err != nil {
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
	req := connect.NewRequest[racestatev1.PublishSpeedmapRequest](p.dataReq)
	withToken(req.Header())
	if _, err := p.r.raceStateService.PublishSpeedmap(p.r.ctx, req); err != nil {
		log.Error("Error publishing speedmap data", log.ErrorField(err))
		return err
	}
	return nil
}

func (r *ReplayTask) Replay(eventId int) error {
	r.providerService = providerv1connect.NewProviderServiceClient(
		http.DefaultClient, Addr, connect.WithGRPC())
	r.raceStateService = racestatev1connect.NewRaceStateServiceClient(
		http.DefaultClient, Addr, connect.WithGRPC())
	r.ctx, r.cancel = context.WithCancel(context.Background())
	defer r.cancel()

	r.stateChan = make(chan *racestatev1.PublishStateRequest)
	r.speedmapChan = make(chan *racestatev1.PublishSpeedmapRequest)
	r.driverDataChan = make(chan *racestatev1.PublishDriverDataRequest)

	var err error
	registerReq := r.dataProvider.ProvideEventData(eventId)

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
	lastTs := time.Time{}
	var selector providerType
	var current peek
	for {
		var delta time.Duration
		var currentIdx int
		// create a max time from  (don't use time.Unix(1<<63-1), that's not what we want)
		nextTs := time.Unix(0, 0).Add(1<<63 - 1)
		for i, p := range pData {
			if p.ts().Before(nextTs) {
				nextTs = p.ts()
				selector = p.provider()
				current = pData[i]
				currentIdx = i
			}
		}

		if !lastTs.IsZero() {
			delta = nextTs.Sub(lastTs)
			if delta > 0 && Speed > 0 {
				wait := time.Duration(int(delta.Nanoseconds()) / Speed)
				log.Debug("Sleeping",
					log.Time("time", nextTs),
					log.Duration("delta", delta),
					log.Duration("wait", wait),
				)
				time.Sleep(wait)
			}
		}
		lastTs = nextTs

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
	req := connect.NewRequest[providerv1.RegisterEventRequest](eventReq)
	withToken(req.Header())
	resp, err := r.providerService.RegisterEvent(r.ctx, req)
	if err == nil {
		return resp.Msg.Event, nil
	}
	return nil, err
}

func (r *ReplayTask) unregisterEvent() error {
	req := connect.NewRequest[providerv1.UnregisterEventRequest](
		&providerv1.UnregisterEventRequest{
			EventSelector: r.buildEventSelector(),
		})
	withToken(req.Header())

	_, err := r.providerService.UnregisterEvent(r.ctx, req)
	return err
}

func (r *ReplayTask) buildEventSelector() *commonv1.EventSelector {
	return &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: r.event.Key}}
}

func withToken(h http.Header) {
	h.Set("api-token", Token)
}
