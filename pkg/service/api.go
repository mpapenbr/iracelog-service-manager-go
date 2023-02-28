package service

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

//nolint:tagliatelle //different structs need to be mapped
type RegisterEventRequest struct {
	EventKey  string              `json:"eventKey"`
	Manifests model.Manifests     `json:"manifests"`
	EventInfo model.EventDataInfo `json:"info"`
	TrackInfo model.TrackInfo     `json:"trackInfo"`
}
