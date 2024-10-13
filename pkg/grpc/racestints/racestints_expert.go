package racestints

import (
	"math"
	"time"
)

type (
	ExpertCalcParams struct {
		RaceDur          time.Duration // duration until end of race
		LC               int           // laps completed
		CurrentStintLaps int           // laps done in current stint
		Lps              int           // laps per stint
		PitTime          time.Duration // complete pit time
		PitLaneTime      time.Duration // time to drive through pit lane
		AvgLap           time.Duration // average lap time
		RefuelRate       float64       // refuel rate in l/s
	}
	Option       func(*expertStintCalc)
	EndOfLapData struct {
		CarInPit      bool          // pit vs track
		StintLap      int           // lap in current stint
		RemainLapTime time.Duration // time to finish current lap
		SessionAtEol  time.Duration // session time at end of lap
	}
	eolComp func() *EndOfLapData
)

type (
	expertStintCalc struct {
		param  *ExpertCalcParams
		parts  []Part
		eolDur eolComp
	}
)

func NewExpertStintCalc(param *ExpertCalcParams, opts ...Option) CalcStints {
	ret := &expertStintCalc{param: param}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithEolDur(eolDur eolComp) func(*expertStintCalc) {
	return func(c *expertStintCalc) {
		c.eolDur = eolDur
	}
}

//nolint:funlen,lll,dupl,nestif // readability,wip
func (c *expertStintCalc) Calc() (*Result, error) {
	stintTime := c.param.AvgLap * time.Duration(c.param.Lps)
	c.parts = make([]Part, 0)
	curLap := c.param.LC
	fillStint := func(sp *stintPart, laps int) {
		sp.lapEnd = curLap + laps - 1
		sp.laps = laps
		sp.stintTime = time.Duration(laps) * c.param.AvgLap
	}
	addPitstop := func() {
		c.parts = append(c.parts, &pitPart{pitTime: c.param.PitTime})
	}
	// we may enter this function when the car is somewhere on the track
	eol := c.eolDur()
	curDur := time.Duration(0) // just init
	// the difficult part: initialization from any possible point in race
	if eol.CarInPit {
		// we need to calc the situation at the end of the current lap
		// curLap += eol.StintLap
	} else {
		curDur = eol.SessionAtEol
		// check how many laps we can do in this stint
		remain := c.param.Lps - eol.StintLap
		if remain > 0 {
			// do remaining laps fit all into race?
			if curDur+(time.Duration(remain)*c.param.AvgLap) < c.param.RaceDur {
				stint := &stintPart{lapStart: curLap}
				fillStint(stint, remain)
				c.parts = append(c.parts, stint)
			} else {
				// calc remaining laps in remaining time and quit
				remain = int(math.Ceil((c.param.RaceDur - curDur).Seconds() / c.param.AvgLap.Seconds()))
				stint := &stintPart{lapStart: curLap}
				fillStint(stint, remain)
				c.parts = append(c.parts, stint)
				return &Result{Parts: c.parts}, nil
			}
		} else {
			addPitstop()
		}
		curLap++
	}

	// the not so difficult part: calc to the end of race

	curStint := &stintPart{lapStart: curLap}

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
