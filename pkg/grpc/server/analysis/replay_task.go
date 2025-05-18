package analysis

import (
	"context"
	"sync"
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

type ReplayDataProvider interface {
	NextDriverData() *racestatev1.PublishDriverDataRequest
	NextStateData() *racestatev1.PublishStateRequest
}
type ReplayOption func(*ReplayTask)

func NewReplayTask(dataProvider ReplayDataProvider, opts ...ReplayOption) *ReplayTask {
	ret := &ReplayTask{dataProvider: dataProvider}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

//nolint:lll // readability
func WithDriverDataCallback(cb func(*racestatev1.PublishDriverDataRequest)) ReplayOption {
	return func(r *ReplayTask) {
		r.driverDataCB = cb
	}
}

func WithStateCallback(cb func(*racestatev1.PublishStateRequest)) ReplayOption {
	return func(r *ReplayTask) {
		r.stateCB = cb
	}
}

type ReplayTask struct {
	dataProvider   ReplayDataProvider
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	stateChan      chan *racestatev1.PublishStateRequest
	driverDataChan chan *racestatev1.PublishDriverDataRequest
	driverDataCB   func(*racestatev1.PublishDriverDataRequest)
	stateCB        func(*racestatev1.PublishStateRequest)
}

func (p *peekDriverData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekDriverData) process() error {
	if p.r.driverDataCB != nil {
		p.r.driverDataCB(p.dataReq)
	}
	return nil
}

func (p *peekStateData) ts() time.Time {
	return p.dataReq.Timestamp.AsTime()
}

func (p *peekStateData) process() error {
	if p.r.stateCB != nil {
		p.r.stateCB(p.dataReq)
	}
	return nil
}

func (r *ReplayTask) Replay() error {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	defer r.cancel()

	r.stateChan = make(chan *racestatev1.PublishStateRequest)
	r.driverDataChan = make(chan *racestatev1.PublishDriverDataRequest)

	var err error

	r.wg = sync.WaitGroup{}
	r.wg.Add(3)
	go r.provideDriverData()
	go r.provideStateData()

	go r.processData()

	log.Debug("Waiting for tasks to finish")
	r.wg.Wait()
	log.Debug("Tasks finished")

	return err
}

//nolint:funlen,gocognit,cyclop //  by design
func (r *ReplayTask) processData() {
	defer r.wg.Done()

	pData := make([]peek, 0, 2)
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
	)
	for _, p := range pData {
		if !p.refill() {
			log.Debug("exhausted", log.String("provider", string(p.provider())))
		}
	}

	var selector providerType
	var current peek
	for {
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

		if err := current.process(); err != nil {
			log.Error("Error processing data", log.ErrorField(err))
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
			log.Debug("Sent data on driverDataChen",
				log.Int("i", i),
				log.Time("ts", item.Timestamp.AsTime()))
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
