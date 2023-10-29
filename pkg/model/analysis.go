package model

type DbAnalysis struct {
	ID      int                 `json:"id"`
	EventID int                 `json:"eventId"`
	Data    AnalysisDataGeneric `json:"data"`
}

// used for json marshaling to be compatible with frontend
type AnalysisDataGeneric map[string]interface{}

type AnalysisData struct {
	CarInfo         []AnalysisCarInfo         `json:"carInfo"`
	CarLaps         []AnalysisCarLaps         `json:"carLaps"`
	CarPits         []AnalysisCarPits         `json:"carPits"`
	CarStints       []AnalysisCarStints       `json:"carStints"`
	CarComputeState []AnalysisCarComputeState `json:"carComputeState"`
	RaceGraph       []AnalysisRaceGraph       `json:"raceGraph"`
	// carNum ordered by race position
	RaceOrder []string `json:"raceOrder"`
}

type AnalysisCarInfo struct {
	Name     string               `json:"name"`
	CarNum   string               `json:"carNum"`
	CarClass string               `json:"carClass"`
	Drivers  []AnalysisDriverInfo `json:"drivers"`
}

type AnalysisDriverInfo struct {
	DriverName string             `json:"driverName"`
	SeatTime   []AnalysisSeatTime `json:"seatTime"`
}

type AnalysisCarLaps struct {
	CarNum string            `json:"carNum"`
	Laps   []AnalysisLapInfo `json:"laps"`
}
type AnalysisLapInfo struct {
	LapNo   int     `json:"lapNo"`
	LapTime float64 `json:"lapTime"`
}
type AnalysisSeatTime struct {
	EnterCarTime float64 `json:"enterCarTime"` // unit: sessionTime
	LeaveCarTime float64 `json:"leaveCarTime"` // unit: sessionTime
}

type AnalysisCarPits struct {
	CarNum  string            `json:"carNum"`
	Current AnalysisPitInfo   `json:"current"`
	History []AnalysisPitInfo `json:"history"`
}
type AnalysisPitInfo struct {
	EnterTime        float64 `json:"enterTime"` // unit: sessionTime
	ExitTime         float64 `json:"exitTime"`  // unit: sessionTime
	CarNum           string  `json:"carNum"`
	LapEnter         int     `json:"lapEnter"`
	LapExit          int     `json:"lapExit"`
	LaneTime         float64 `json:"laneTime"`
	IsCurrentPitstop bool    `json:"isCurrentPitstop"`
}

type AnalysisCarStints struct {
	CarNum  string              `json:"carNum"`
	Current AnalysisStintInfo   `json:"current"`
	History []AnalysisStintInfo `json:"history"`
}
type AnalysisStintInfo struct {
	EnterTime      float64 `json:"enterTime"` // unit: sessionTime
	ExitTime       float64 `json:"exitTime"`  // unit: sessionTime
	CarNum         string  `json:"carNum"`
	LapEnter       int     `json:"lapEnter"`
	LapExit        int     `json:"lapExit"`
	StintTime      float64 `json:"stintTime"`
	NumLaps        int     `json:"numLaps"`
	IsCurrentStint bool    `json:"isCurrentStint"`
}

type AnalysisRaceGraph struct {
	LapNo    int               `json:"lapNo"`
	CarClass string            `json:"carClass"`
	Gaps     []AnalysisGapInfo `json:"gaps"`
}
type AnalysisGapInfo struct {
	CarNum string  `json:"carNum"`
	LapNo  int     `json:"lapNo"`
	Gap    float64 `json:"gap"`
	Pos    int     `json:"pos"`
	Pic    int     `json:"pic"`
}

type AnalysisCarComputeState struct {
	CarNum string `json:"carNum"`
	State  string `json:"state"`
	// sessionTime when state switched from PIT/RUN to OUT
	OutEncountered float64 `json:"outEncountered"`
}
type DbTeamInEvent struct {
	Name     string `json:"name"`
	CarNum   string `json:"carNum"`
	CarClass string `json:"carClass"`
	Drivers  []struct {
		DriverName string `json:"driverName"`
	}
}
