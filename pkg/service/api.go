package service

import "github.com/mpapenbr/iracelog-service-manager-go/pkg/model"

type RegisterEventRequest struct {
	EventKey  string              `json:"eventKey"`
	Manifests model.Manifests     `json:"manifests"`
	EventInfo model.EventDataInfo `json:"eventInfo"`
	TrackInfo model.TrackInfo     `json:"trackInfo"`
}
