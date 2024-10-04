package racestints

import "time"

type (
	PartType   int
	CalcStints interface {
		Calc() (*Result, error)
	}
	Part interface {
		Type() PartType
		Output() string
	}
	StintPart interface {
		Part
		Laps() int
		LapStart() int
		LapEnd() int
		StintTime() time.Duration
	}
	PitPart interface {
		Part
		PitTime() time.Duration
	}
	Result struct {
		Parts []Part
	}
)

const (
	PartTypeStint PartType = iota
	PartTypePit
)
