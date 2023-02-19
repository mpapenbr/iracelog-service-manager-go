package model

import "time"

type DbEvent struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string
	RecordStamp time.Time `json:"recordStamp"`
	Info        struct {
		TrackId           int    `json:"trackId"`
		EventTime         string `json:"eventTime"`
		RaceloggerVersion string `json:"raceloggerVersion"`
		TeamRacing        int    `json:"teamRacing"` // 0: false
		MultiClass        bool   `json:"multiClass"`
		NumCarTypes       int    `json:"numCarTypes"`
		NumCarClasses     int    `json:"numCarClasses"`
		IrSessionId       int    `json:"irSessionId"`
		Sessions          []struct {
			Num  int    `json:"num"`
			Name string `json:"name"`
		}
	}
	Manifests struct {
		Car     []string `json:"car"`
		Pit     []string `json:"pit"`
		Message []string `json:"message"`
		Session []string `json:"session"`
	}
	ReplayInfo struct {
		MinTimestamp   float64 `json:"minTimestamp"`
		MinSessionTime float64 `json:"minSessionTime"`
		MaxSessionTime float64 `json:"maxSessionTime"`
	}
}
