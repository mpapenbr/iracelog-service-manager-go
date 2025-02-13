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
}

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
	log.Info("Closing broadcast server",
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
	}
	b.setupMetrics()
	go b.serve()
	return b
}

//nolint:lll,funlen // readability
func (b *broadcastServer[T]) setupMetrics() {
	// Implement the setupMetrics method here
	// For example, initialize metrics or logging
	log.Info("Setting up metrics",
		log.String("eventKey", b.eventKey),
		log.String("name", b.name))
	meter := otel.GetMeterProvider().Meter(fmt.Sprintf("ism.broadcast.%s", b.name))
	register := func(metricName, desc, unit string, valueProvider func() int64) {
		if _, err := meter.Int64ObservableGauge(
			metricName,
			metric.WithDescription(desc),
			metric.WithUnit(unit),

			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(valueProvider(),
					metric.WithAttributes(
						attribute.String("name", b.name),
						attribute.String("event", b.eventKey),
					),
				)
				return nil
			})); err != nil {
			log.Error("failed to register metric",
				log.String("metric", metricName),
				log.ErrorField(err))
		}
	}
	type data struct {
		name  string
		desc  string
		unit  string
		value func() int64
	}
	for _, d := range []*data{
		{
			"ism.broadcast.rcv", "Number of received messages", "{count}",
			func() int64 { return int64(b.numRcv) },
		},
		{
			"ism.broadcast.snd", "Number of sent messages", "{count}",
			func() int64 { return int64(b.numSnd) },
		},
		{
			"ism.broadcast.skip", "Number of skipped messages", "{count}",
			func() int64 { return int64(b.numSkip) },
		},
		{
			"ism.broadcast.listener", "Number of listeners", "{count}",
			func() int64 { return int64(len(b.listeners)) },
		},
	} {
		register(d.name, d.desc, d.unit, d.value)
	}
}

//nolint:funlen,cyclop,gocognit // by design
func (b *broadcastServer[T]) serve() {
	defer func() {
		log.Info("Closing listeners", log.String("name", b.name))
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
			log.Info("broadcast server about to be closed", log.String("name", b.name))
			return
		case ch := <-b.addListener:
			b.listeners = append(b.listeners, ch)
		case ch := <-b.removeListener:
			log.Debug("removing listener",
				log.String("name", b.name), log.Int("len", len(b.listeners)))
			m.Lock()
			log.Debug("got lock")
			for i, listener := range b.listeners {
				if listener == ch {
					b.listeners = append(b.listeners[:i], b.listeners[i+1:]...)

					log.Debug("removed listener",
						log.String("name", b.name), log.Int("len", len(b.listeners)))

					close(listener)
					log.Debug("closed channel",
						log.String("name", b.name))
					// break
				}
			}
			log.Debug("unlocking",
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
