package broadcast

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

//nolint:lll // by design
// see https://betterprogramming.pub/how-to-broadcast-messages-in-go-using-channels-b68f42bdf32e

type BroadcastServer[T any] interface {
	Subscribe() <-chan T
	CancelSubscription(<-chan T)
	Close()
}

type broadcastServer[T any] struct {
	name           string
	source         <-chan T
	listeners      []chan T
	addListener    chan chan T
	removeListener chan (<-chan T)
	ctx            context.Context
	cancel         context.CancelFunc
	numRcv         int
	numSnd         int
	numSkip        int
	eventKey       string
	mData          *mData
	l              *log.Logger
}

// check interface compliance
var _ BroadcastServer[any] = (*broadcastServer[any])(nil)

type Option[T any] func(*broadcastServer[T])

func WithTelemetry[T any](eventKey string) Option[T] {
	return func(b *broadcastServer[T]) {
		b.eventKey = eventKey
	}
}

func (b *broadcastServer[T]) Subscribe() <-chan T {
	ch := make(chan T)
	b.addListener <- ch
	return ch
}

func (b *broadcastServer[T]) CancelSubscription(ch <-chan T) {
	b.removeListener <- ch
}

func (b *broadcastServer[T]) Close() {
	b.l.Info("Closing broadcast server",
		log.String("name", b.name),
		log.Int("rcv", b.numRcv), log.Int("snd", b.numSnd), log.Int("skip", b.numSkip))
	b.cancel()
}

//nolint:whitespace // false positive
func NewBroadcastServer[T any](
	eventKey, name string,
	source <-chan T,
) BroadcastServer[T] {
	ctx, cancel := context.WithCancel(context.Background())
	b := &broadcastServer[T]{
		eventKey:       eventKey,
		name:           name,
		source:         source,
		addListener:    make(chan chan T),
		removeListener: make(chan (<-chan T)),
		ctx:            ctx,
		cancel:         cancel,
		l:              log.Default().Named(fmt.Sprintf("bcst.%s", name)),
	}
	b.setupMetrics()
	go b.serve()
	return b
}

type (
	bCstMetricInfo struct {
		metricName string
		gauge      metric.Int64ObservableGauge
		reg        metric.Registration
	}
	mData struct {
		meter                    metric.Meter
		rcv, snd, skip, listener *bCstMetricInfo
	}
)

var myMeter = otel.GetMeterProvider().Meter("ism.broadcast")

func (b *broadcastServer[T]) createMData(meter metric.Meter) (*mData, error) {
	ret := &mData{meter: meter}

	type data struct {
		name, desc, unit string
		assign           func(metricName string, gauge metric.Int64ObservableGauge)
	}
	for _, d := range []*data{
		{
			"ism.broadcast.rcv", "Number of received messages", "{count}",
			func(metricName string, gauge metric.Int64ObservableGauge) {
				ret.rcv = &bCstMetricInfo{metricName: metricName, gauge: gauge}
			},
		},
		{
			"ism.broadcast.snd", "Number of sent messages", "{count}",
			func(metricName string, gauge metric.Int64ObservableGauge) {
				ret.snd = &bCstMetricInfo{metricName: metricName, gauge: gauge}
			},
		},
		{
			"ism.broadcast.skip", "Number of skipped messages", "{count}",
			func(metricName string, gauge metric.Int64ObservableGauge) {
				ret.skip = &bCstMetricInfo{metricName: metricName, gauge: gauge}
			},
		},
		{
			"ism.broadcast.listener", "Number of listeners", "{count}",
			func(metricName string, gauge metric.Int64ObservableGauge) {
				ret.listener = &bCstMetricInfo{metricName: metricName, gauge: gauge}
			},
		},
	} {
		if gauge, err := meter.Int64ObservableGauge(
			d.name,
			metric.WithDescription(d.desc),
			metric.WithUnit(d.unit),
		); err != nil {
			return nil, err
		} else {
			d.assign(d.name, gauge)
		}
	}
	return ret, nil
}

//nolint:lll,funlen // readability
func (b *broadcastServer[T]) setupMetrics() {
	b.l.Info("Setting up metrics",
		log.String("eventKey", b.eventKey),
		log.String("name", b.name))
	var err error
	var mData *mData
	if mData, err = b.createMData(myMeter); err != nil {
		b.l.Error("failed to create metric",
			log.String("eventKey", b.eventKey),
			log.String("name", b.name),
			log.ErrorField(err))
		return
	} else {
		b.mData = mData
	}

	type data struct {
		info  *bCstMetricInfo
		value func() int64
	}
	for _, d := range []*data{
		{
			mData.rcv,
			func() int64 { return int64(b.numRcv) },
		},
		{
			mData.listener,
			func() int64 { return int64(len(b.listeners)) },
		},
		{
			mData.snd,
			func() int64 { return int64(b.numSnd) },
		},
		{
			mData.skip,
			func() int64 { return int64(b.numSkip) },
		},
	} {
		var err error
		d.info.reg, err = myMeter.RegisterCallback(
			func(_ context.Context, o metric.Observer) error {
				b.l.Debug("calling metric callback",
					log.String("metric", d.info.metricName),
					log.String("name", b.name),
					log.String("event", b.eventKey))
				o.ObserveInt64(d.info.gauge, d.value(),
					metric.WithAttributes(
						attribute.String("name", b.name),
						attribute.String("event", b.eventKey),
					),
				)
				return nil
			},
			d.info.gauge,
		)
		if err != nil {
			b.l.Error("failed to register metric callback",
				log.String("metric", d.info.metricName),
				log.ErrorField(err))
		} else {
			b.l.Debug("registered metric callback",
				log.String("metric", d.info.metricName),
				log.String("name", b.name),
				log.String("event", b.eventKey))
		}
	}
}

//nolint:funlen,cyclop,gocognit // by design
func (b *broadcastServer[T]) serve() {
	defer func() {
		b.l.Info("Closing listeners", log.String("name", b.name))
		for _, listener := range b.listeners {
			if listener != nil {
				close(listener)
			}
		}
	}()
	m := sync.Mutex{}
	for {
		select {
		case <-b.ctx.Done():
			b.l.Info("broadcast server about to be closed", log.String("name", b.name))
			b.unregisterMetrics()
			return
		case ch := <-b.addListener:
			b.listeners = append(b.listeners, ch)
		case ch := <-b.removeListener:
			b.l.Debug("removing listener",
				log.String("name", b.name), log.Int("len", len(b.listeners)))
			m.Lock()
			b.l.Debug("got lock")
			for i, listener := range b.listeners {
				if listener == ch {
					b.listeners = append(b.listeners[:i], b.listeners[i+1:]...)

					b.l.Debug("removed listener",
						log.String("name", b.name), log.Int("len", len(b.listeners)))

					close(listener)
					b.l.Debug("closed channel",
						log.String("name", b.name))
					// break
				}
			}
			b.l.Debug("unlocking",
				log.String("name", b.name), log.Int("len", len(b.listeners)))
			m.Unlock()
		case msg := <-b.source:
			m.Lock()
			b.numRcv++

			for _, listener := range b.listeners {
				select {
				case listener <- msg:
					b.numSnd++
				// as an alternative we could use a timeout here
				// something like case <-time.After(100 * time.Millisecond):
				// but we have to keep in mind: don't wait too long
				case <-time.After(50 * time.Millisecond):
					// default:
					// log.Debug("skipping listener", log.Int("len", len(b.listeners)))
					b.numSkip++
				}
			}

			m.Unlock()
		}
	}
}

func (b *broadcastServer[T]) unregisterMetrics() {
	for _, info := range []*bCstMetricInfo{
		b.mData.rcv, b.mData.snd, b.mData.skip, b.mData.listener,
	} {
		if info.reg != nil {
			if err := info.reg.Unregister(); err != nil {
				b.l.Error("failed to unregister metric",
					log.String("metric", info.metricName),
					log.ErrorField(err))
			} else {
				b.l.Debug("unregistered metric",
					log.String("metric", info.metricName),
					log.String("name", b.name),
					log.String("event", b.eventKey))
			}
		} else {
			b.l.Debug("no registration to unregister",
				log.String("metric", info.metricName),
				log.String("name", b.name),
				log.String("event", b.eventKey))
		}
	}
}
