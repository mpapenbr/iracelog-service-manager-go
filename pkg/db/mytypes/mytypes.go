package mytypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
)

//nolint:tagliatelle // json is that way
type (
	// we need this extra type to be usable with bob generated code
	EventSessionSlice []*eventv1.Session

	// we need this extra type to be usable with bob generated code
	SectorSlice []*trackv1.Sector

	// we need this extra type to be usable with bob generated code
	TireInfoSlice []*eventv1.TireInfo

	CustomExtraInfo struct {
		PitInfo *trackv1.PitInfo `json:"pit_info"`
	}
)

func (h *EventSessionSlice) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value is not []byte")
	}

	return json.Unmarshal(bytes, &h)
}

func (h EventSessionSlice) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *SectorSlice) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value is not []byte")
	}

	return json.Unmarshal(bytes, &h)
}

func (h SectorSlice) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *TireInfoSlice) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value is not []byte")
	}
	return json.Unmarshal(bytes, &h)
}

func (h TireInfoSlice) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *CustomExtraInfo) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value is not []byte")
	}

	return json.Unmarshal(bytes, &h)
}

func (h CustomExtraInfo) Value() (driver.Value, error) {
	return json.Marshal(h)
}
