package util

import (
	"strings"

	"golang.org/x/mod/semver"
)

const (
	RequiredClientVersion string = "v0.20.0"
)

func CheckRaceloggerVersion(toCheck string) bool {
	if !strings.HasPrefix(toCheck, "v") {
		toCheck = "v" + toCheck
	}
	res := semver.Compare(toCheck, RequiredClientVersion)
	return res >= 0
}
