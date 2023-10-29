package model

type DbCar struct {
	ID      int     `json:"id"`
	EventID int     `json:"eventId"`
	Data    CarData `json:"data"`
}

type CarData struct {
	Type      int        `json:"type"`
	Payload   CarPayload `json:"payload"`
	Timestamp float64    `json:"timestamp"`
}

type CarPayload struct {
	Cars       []CarInfo  `json:"cars"`
	CarClasses []CarClass `json:"carClasses"`
	Entries    []CarEntry `json:"entries"`
	// map of carIdx to driver name (for current driver)
	// TODO: racelogger must be changed to send this data
	CurrentDrivers map[int]string `json:"currentDrivers"`
	SessionTime    float64        `json:"sessionTime"`
}

type CarInfo struct {
	Name          string  `json:"name"`
	NameShort     string  `json:"nameShort"`
	CarID         int     `json:"carId"`
	CarClassID    int     `json:"carClassId"`
	CarClassName  string  `json:"carClassName"`
	FuelPct       float64 `json:"fuelPct"`
	PowerAdjust   float64 `json:"powerAdjust"`
	WeightPenalty float64 `json:"weightPenalty"`
	DryTireSets   int     `json:"dryTireSets"`
}

type CarClass struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CarEntry struct {
	Car     Car      `json:"car"`
	Team    Team     `json:"team"`
	Drivers []Driver `json:"drivers"`
}

type Car struct {
	CarID        int    `json:"carId"`
	CarIdx       int    `json:"carIdx"`
	CarClassID   int    `json:"carClassId"`
	Name         string `json:"name"`
	CarNumber    string `json:"carNumber"`
	CarNumberRaw int    `json:"carNumberRaw"`
}

type Team struct {
	ID     int    `json:"id"`
	CarIdx int    `json:"carIdx"`
	Name   string `json:"name"`
}
type Driver struct {
	ID          int    `json:"id"`
	CarIdx      int    `json:"carIdx"`
	Name        string `json:"name"`
	IRating     int    `json:"iRating"`
	Initials    string `json:"initials"`
	LicLevel    int    `json:"licLevel"`
	LicSubLevel int    `json:"licSubLevel"`
	LicString   string `json:"licString"`
	AbbrevName  string `json:"abbrevName"`
}
