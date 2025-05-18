package analysis

import (
	"time"
)

type (
	myLoaderFunc[E any] func() ([]*E, error)
)

//nolint:unused // false positive
type commonFetcher[E any] struct {
	loader myLoaderFunc[E]
	buffer []*E
	lastTS time.Time
}

//nolint:unused // false positive
func (f *commonFetcher[E]) next() *E {
	if len(f.buffer) == 0 {
		f.fetch()
	}
	if len(f.buffer) == 0 {
		return nil
	}
	ret := f.buffer[0]
	f.buffer = f.buffer[1:]

	return ret
}

//nolint:unused // false positive
func (f *commonFetcher[E]) fetch() {
	f.buffer, _ = f.loader()
}
