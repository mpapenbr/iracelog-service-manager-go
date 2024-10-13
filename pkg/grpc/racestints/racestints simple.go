package racestints

import (
	"fmt"
	"math"
	"time"
)

type (
	SimpleCalcParams struct {
		RaceDur time.Duration // duration until end of race
		Lps     int           // laps per stint
		PitTime time.Duration // pit time
		AvgLap  time.Duration // average lap time
	}
)

type (
	simpleStintCalc struct {
		param *SimpleCalcParams
		parts []Part
	}
	stintPart struct {
		laps      int
		lapStart  int
		lapEnd    int
		stintTime time.Duration
	}
	pitPart struct {
		pitTime time.Duration
	}
)

func NewSimpleStintCalc(param *SimpleCalcParams) CalcStints {
	return &simpleStintCalc{param: param}
}

//nolint:funlen,lll,dupl,nestif // readability,wip
func (c *simpleStintCalc) Calc() (*Result, error) {
	stintTime := c.param.AvgLap * time.Duration(c.param.Lps)
	c.parts = make([]Part, 0)
	curLap := 1
	curDur := time.Duration(0)
	curStint := &stintPart{lapStart: curLap}
	fillStint := func(sp *stintPart, laps int) {
		sp.lapEnd = curLap + laps - 1
		sp.laps = laps
		sp.stintTime = time.Duration(laps) * c.param.AvgLap
	}

	for curDur < c.param.RaceDur {
		if curDur+stintTime < c.param.RaceDur {
			fillStint(curStint, c.param.Lps)
			c.parts = append(c.parts, curStint)
			curLap += c.param.Lps
			curDur += stintTime
			c.parts = append(c.parts, &pitPart{pitTime: c.param.PitTime})
			curDur += c.param.PitTime
			curStint = &stintPart{lapStart: curLap}
			// after the pitstop the race is over.
			// nevertheless we need another stint to finish the race
			if curDur >= c.param.RaceDur {
				curStint.laps = 1
				curStint.lapEnd = curLap
				curStint.stintTime = c.param.AvgLap
			}
		} else {
			remain := c.param.RaceDur - curDur
			laps := int(math.Ceil(remain.Seconds() / c.param.AvgLap.Seconds()))
			fillStint(curStint, laps)
			curDur += curStint.stintTime
		}
	}
	c.parts = append(c.parts, curStint)
	return &Result{Parts: c.parts}, nil
}

func (s stintPart) Type() PartType {
	return PartTypeStint
}

func (s stintPart) Laps() int {
	return s.laps
}

func (s stintPart) LapStart() int {
	return s.lapStart
}

func (s stintPart) LapEnd() int {
	return s.lapEnd
}

func (s stintPart) StintTime() time.Duration {
	return s.stintTime
}

func (s stintPart) Output() string {
	return fmt.Sprintf("%d-%d (%d): %s", s.lapStart, s.lapEnd, s.laps, s.stintTime)
}

func (p pitPart) Type() PartType {
	return PartTypePit
}

func (p pitPart) PitTime() time.Duration {
	return p.pitTime
}

func (p pitPart) Output() string {
	return fmt.Sprintf("Pit %s", p.pitTime)
}
