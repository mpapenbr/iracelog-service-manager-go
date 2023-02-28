package provider

import "github.com/mpapenbr/iracelog-service-manager-go/pkg/model"

type ManagerCmdType int

const (
	Register ManagerCmdType = 1
	Removed  ManagerCmdType = 2
)

type ManagerCommand struct {
	Type ManagerCmdType `json:"type"`
}

type PublishNew struct {
	Type    ManagerCmdType     `json:"type"`
	Payload NewProviderPayload `json:"payload"`
}

type NewProviderPayload struct {
	EventKey  string              `json:"eventKey"`
	Manifests model.Manifests     `json:"manifests"`
	Info      model.EventDataInfo `json:"info"`
}

type PublishRemoved struct {
	Type    ManagerCmdType `json:"type"`
	Payload string         `json:"payload"` // contains the event key
}
