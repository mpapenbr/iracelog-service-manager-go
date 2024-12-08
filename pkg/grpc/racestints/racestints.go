package racestints

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
)

type (
	PartType       int
	CalcType       int
	BaseCalcParams interface {
		SetRaceDur(time.Duration)
		SetLps(int)
		SetPitTime(time.Duration)
		SetAvgLap(time.Duration)
	}
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

const (
	CalcTypeSimple CalcType = iota
	CalcTypeExpert
)

const (
	Unknown = "unknown"
)

var ErrUnmarshalNil = errors.New("can't unmarshal a nil value")

func ParseCalcType(text string) (CalcType, error) {
	var f CalcType
	err := f.UnmarshalText([]byte(text))
	return f, err
}

func (c CalcType) String() string {
	switch c {
	case CalcTypeSimple:
		return "simple"
	case CalcTypeExpert:
		return "expert"
	default:
		return Unknown
	}
}

func (c CalcType) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c *CalcType) UnmarshalText(text []byte) error {
	if c == nil {
		return ErrUnmarshalNil
	}
	if !c.unmarshalText(text) && !c.unmarshalText(bytes.ToLower(text)) {
		return fmt.Errorf("unrecognized format: %q", text)
	}
	return nil
}

func (c *CalcType) unmarshalText(text []byte) bool {
	switch strings.ToLower(string(text)) {
	case "simple", "":
		*c = CalcTypeSimple
	case "expert":
		*c = CalcTypeExpert
	default:
		return false
	}
	return true
}
