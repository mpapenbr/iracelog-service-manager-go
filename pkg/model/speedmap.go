package model

type DbSpeedmap struct {
	ID      int          `json:"id"`
	EventID int          `json:"eventId"`
	Data    SpeedmapData `json:"data"`
}

type SpeedmapData struct {
	Type      int             `json:"type"`
	Payload   SpeedmapPayload `json:"payload"`
	Timestamp float64         `json:"timestamp"`
}

type SpeedmapPayload struct {
	Data        map[string]ClassSpeedmapData `json:"data"`
	ChunkSize   int                          `json:"chunkSize"`
	TimeOfDay   float64                      `json:"timeOfDay"`
	TrackTemp   float64                      `json:"trackTemp"`
	TrackLength float64                      `json:"trackLength"`
	SessionTime float64                      `json:"sessionTime"`
	CurrentPos  float64                      `json:"currentPos"`
}

type ClassSpeedmapData struct {
	Laptime     float64   `json:"laptime"`
	ChunkSpeeds []float64 `json:"chunkSpeeds"`
}
