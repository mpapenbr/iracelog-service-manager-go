package util

import (
	"time"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
)

type providerType string

const (
	DriverData   providerType = "DriverData"
	StateData    providerType = "StateData"
	SpeedmapData providerType = "SpeedmapData"
)

type peek interface {
	ts() time.Time
	provider() providerType
	publish() error
	refill() bool
}
type commonStateData[E any] struct {
	dataChan     chan *E
	dataReq      *E
	r            *ReplayTask
	providerType providerType
}

func (p *commonStateData[E]) refill() bool {
	var ok bool
	p.dataReq, ok = <-p.dataChan
	return ok
}

func (p *commonStateData[E]) provider() providerType {
	return p.providerType
}

type peekStateData struct {
	commonStateData[racestatev1.PublishStateRequest]
}
type peekSpeedmapData struct {
	commonStateData[racestatev1.PublishSpeedmapRequest]
}
type peekDriverData struct {
	commonStateData[racestatev1.PublishDriverDataRequest]
}
