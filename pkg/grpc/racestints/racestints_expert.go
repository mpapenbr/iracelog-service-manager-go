package racestints

import (
	"math"
	"time"
)

type (
	ExpertCalcParams struct {
		RaceDur          time.Duration // duration until end of race
		LC               int           // laps completed
		Lps              int           // laps per stint
		PitTime          time.Duration // complete pit time
		PitLaneTime      time.Duration // time to drive through pit lane
		AvgLap           time.Duration // average lap time
		RefuelRate       float64       // refuel rate in l/s
		EndOfLapDataFunc func() *EndOfLapData
	}
	Option       func(*expertStintCalc)
	EndOfLapData struct {
		CarInPit bool // pit vs track
		StintLap int  // lap in current stint
		// RemainLapTime time.Duration // time to finish current lap
		SessionAtEol time.Duration // session time at end of lap
	}
	EndOfLapDataOption func(*EndOfLapData)
	eolComp            func() *EndOfLapData
)

type (
	expertStintCalc struct {
		param *ExpertCalcParams
		parts []Part
	}
)

func NewEndOfLapData(opts ...EndOfLapDataOption) *EndOfLapData {
	ret := &EndOfLapData{
		CarInPit: false,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func WithCarInPit(arg bool) EndOfLapDataOption {
	return func(e *EndOfLapData) {
		e.CarInPit = arg
	}
}

func WithStintLap(arg int) EndOfLapDataOption {
	return func(e *EndOfLapData) {
		e.StintLap = arg
	}
}

func WithSessionAtEol(arg time.Duration) EndOfLapDataOption {
	return func(e *EndOfLapData) {
		e.SessionAtEol = arg
	}
}

func NewExpertStintCalc(param *ExpertCalcParams, opts ...Option) CalcStints {
	ret := &expertStintCalc{param: param}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// calc stints includig pitstops to the end of the race
// Note: the calculation starts at the end of the current lap

//nolint:funlen,lll,dupl,nestif // readability,wip
func (c *expertStintCalc) Calc() (*Result, error) {
	stintTime := c.param.AvgLap * time.Duration(c.param.Lps)
	c.parts = make([]Part, 0)
	// curLap is the lap when the stint computation starts
	// LC is the number of laps completed
	// we add +2 because
	// +1 for the current lap (this is our internal fast forward to the next reference point)
	// +1 to have the lap number of the for the next lap
	curLap := c.param.LC + 2
	fillStint := func(sp *stintPart, laps int) {
		sp.lapEnd = sp.lapStart + laps - 1
		sp.laps = laps
		sp.stintTime = time.Duration(laps) * c.param.AvgLap
	}
	addPitstop := func() {
		c.parts = append(c.parts, &pitPart{pitTime: c.param.PitTime})
	}
	// we may enter this function when the car is somewhere on the track
	eol := c.param.EndOfLapDataFunc()
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
				curLap += remain
				curDur += time.Duration(remain)*c.param.AvgLap + c.param.PitTime
				addPitstop()
			} else {
				// calc remaining laps in remaining time and quit
				remain = int(math.Ceil((c.param.RaceDur - curDur).Seconds() / c.param.AvgLap.Seconds()))
				if remain == 0 {
					return &Result{Parts: c.parts}, nil
				}
				stint := &stintPart{lapStart: curLap}
				fillStint(stint, remain)
				c.parts = append(c.parts, stint)
				return &Result{Parts: c.parts}, nil
			}
		} else {
			// the end of the lap would be the last of that stint
			addPitstop()

			curDur += c.param.PitTime
			// special case: the race duration ends while the car is in the pit
			// we need to add another lap
			if curDur >= c.param.RaceDur {
				stint := &stintPart{lapStart: curLap + 1}
				fillStint(stint, 1)
				c.parts = append(c.parts, stint)
				return &Result{Parts: c.parts}, nil
			}
		}
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
			remainDur := c.param.RaceDur - curDur
			laps := int(math.Ceil(remainDur.Seconds() / c.param.AvgLap.Seconds()))
			fillStint(curStint, laps)
			curDur += curStint.stintTime
		}
	}
	c.parts = append(c.parts, curStint)
	return &Result{Parts: c.parts}, nil
}
