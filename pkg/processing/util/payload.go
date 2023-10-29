package util

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

type PayloadExtractor struct {
	Manifests        *model.Manifests
	CarKeyLookup     map[string]int
	SessionKeyLookup map[string]int
}

func NewPayloadExtractor(manifests *model.Manifests) *PayloadExtractor {
	createLookup := func(manifestKeys []string) map[string]int {
		ret := make(map[string]int)
		for i, v := range manifestKeys {
			ret[v] = i
		}
		return ret
	}

	ret := &PayloadExtractor{
		Manifests:        manifests,
		CarKeyLookup:     createLookup(manifests.Car),
		SessionKeyLookup: createLookup(manifests.Session),
	}
	return ret
}

//nolint:whitespace // can't make the linters happy
func (p *PayloadExtractor) ExtractCarValue(
	rawData []interface{}, key string,
) interface{} {
	return rawData[p.CarKeyLookup[key]]
}

//nolint:whitespace // can't make the linters happy
func (p *PayloadExtractor) ExtractSessionValue(
	rawData []interface{}, key string,
) interface{} {
	return rawData[p.SessionKeyLookup[key]]
}
