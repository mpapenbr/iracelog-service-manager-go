package util

import (
	"time"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
)

func ParseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}

func CollectReplayOptions() []ReplayOption {
	opts := []ReplayOption{}
	if FastForward != "" {
		if dur, err := time.ParseDuration(FastForward); err == nil {
			opts = append(opts, WithFastForward(dur))
		}
	}
	return opts
}
