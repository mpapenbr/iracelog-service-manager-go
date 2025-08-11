package mytypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
)

//nolint:tagliatelle // json is that way
type (
	// we need this extra type to be usable with bob generated code
	EventSessionSlice []EventSession
	EventSession      struct {
		Num         int    `json:"num"`
		Name        string `json:"name"`
		Type        int    `json:"type"`
		SessionTime int    `json:"session_time"`
		Laps        int    `json:"laps"`
	}
	// we need this extra type to be usable with bob generated code
	SectorSlice []Sector
	Sector      struct {
		Num      int     `json:"num"`
		StartPct float64 `json:"start_pct"`
	}
	// we need this extra type to be usable with bob generated code
	TireInfoSlice []TireInfo
	TireInfo      struct {
		Index        int    `json:"index"`
		CompoundType string `json:"compound_type"`
	}
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
