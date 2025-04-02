package util

import (
	"time"
)

type RangeContainer[E any] struct {
	Data            []*E
	LastTimestamp   time.Time
	LastSessionTime float32
	LastRsInfoID    int
}
