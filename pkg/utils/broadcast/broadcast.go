package broadcast

import (
	"context"

	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

//nolint:lll // by design
// see https://betterprogramming.pub/how-to-broadcast-messages-in-go-using-channels-b68f42bdf32e

type BroadcastServerOld interface {
	Subscribe() <-chan *analysisv1.Analysis
	CancelSubscription(<-chan *analysisv1.Analysis)
}
type BroadcastServer[T any] interface {
	Subscribe() <-chan T
	CancelSubscription(<-chan T)
	Close()
}

type broadcastServer[T any] struct {
	source         <-chan T
	listeners      []chan T
	addListener    chan chan T
	removeListener chan (<-chan T)
	ctx            context.Context
	cancel         context.CancelFunc
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
	log.Info("Closing broadcast server")
	b.cancel()
}

func NewBroadcastServer[T any](source <-chan T) BroadcastServer[T] {
	ctx, cancel := context.WithCancel(context.Background())
	b := &broadcastServer[T]{
		source:         source,
		addListener:    make(chan chan T),
		removeListener: make(chan (<-chan T)),
		ctx:            ctx,
		cancel:         cancel,
	}

	go b.serve()
	return b
}

func (b *broadcastServer[T]) serve() {
	defer func() {
		log.Info("Closing listeners")
		for _, listener := range b.listeners {
			if listener != nil {
				close(listener)
			}
		}
	}()
	for {
		select {
		case <-b.ctx.Done():
			log.Info("broadcast server about to be closed")
			return
		case ch := <-b.addListener:
			b.listeners = append(b.listeners, ch)
		case ch := <-b.removeListener:
			for i, listener := range b.listeners {
				if listener == ch {
					b.listeners = append(b.listeners[:i], b.listeners[i+1:]...)
					break
				}
			}
		case msg := <-b.source:
			for _, listener := range b.listeners {
				listener <- msg
			}
		}
	}
}
