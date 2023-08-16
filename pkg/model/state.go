package model

// describes state of the race according to event manifests
type DbState struct {
	ID      int       `json:"id"`
	EventID int       `json:"eventId"`
	Data    StateData `json:"data"`
}

// contains the state message sent by the client via WAMP
type StateData struct {
	Type      int          `json:"type"`
	Payload   StatePayload `json:"payload"`
	Timestamp float64      `json:"timestamp"`
}

// these attributes contain generic data according to event manifests
type StatePayload struct {
	Cars     [][]interface{} `json:"cars"`
	Session  []interface{}   `json:"session"`
	Messages [][]interface{} `json:"messages"`
}

type StateDelta struct {
	Type      int          `json:"type"`
	Payload   DeltaPayload `json:"payload"`
	Timestamp float64      `json:"timestamp"`
}

type DeltaPayload struct {
	Cars    [][3]interface{} `json:"cars"`
	Session [][2]interface{} `json:"session"`
}
