package model

type DbAnalysis struct {
	ID      int          `json:"id"`
	EventID int          `json:"eventId"`
	Data    AnalysisData `json:"data"`
}

type AnalysisData map[string]interface{}

type AnalysisCarInfo struct {
	Name     string `json:"name"`
	CarNum   string `json:"carNum"`
	CarClass string `json:"carClass"`
	Drivers  []struct {
		SeatTime []struct {
			EnterCarTime float64 // unit: sessionTime
			LeaveCarTime float64 // unit: sessionTime
		}
		DriverName string `json:"driverName"`
	}
}

type DbTeamInEvent struct {
	Name     string `json:"name"`
	CarNum   string `json:"carNum"`
	CarClass string `json:"carClass"`
	Drivers  []struct {
		DriverName string `json:"driverName"`
	}
}
