package util

import "github.com/mpapenbr/iracelog-service-manager-go/log"

func ParseLogLevel(l string, defaultVal log.Level) log.Level {
	level, err := log.ParseLevel(l)
	if err != nil {
		return defaultVal
	}
	return level
}
