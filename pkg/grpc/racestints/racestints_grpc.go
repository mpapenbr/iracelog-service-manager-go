package racestints

import (
	"math"
	"time"

	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// calc stints includig pitstops to the end of the race
// Note: the calculation starts at the end of the current lap

//nolint:funlen,lll,dupl,nestif,gocritic,gocyclo // readability,wip
func Calc(c *predictv1.PredictParam) (*predictv1.PredictResult, error) {
	avgLap := c.Stint.AvgLaptime.AsDuration()
	stintTime := avgLap * time.Duration(c.Stint.Lps)
	pitTime := c.Pit.Overall.AsDuration()
	parts := make([]*predictv1.Part, 0)
	// curLap is the lap when the stint computation starts
	curLap := int32(1)
	raceStarted := c.Race.Lc != 0 || c.Car.StintLap != 0
	if raceStarted {
		// LC is the number of laps completed
		// we add +2 because
		// +1 for the current lap (this is our internal fast forward to the next reference point)
		// +1 to have the lap number of the for the next lap
		curLap = c.Race.Lc + 2
	}
	var sessionOffset time.Duration = 0
	if c.Race.Session != nil {
		sessionOffset = c.Race.Session.AsDuration()
	}

	createPart := func(start, duration time.Duration) *predictv1.Part {
		return &predictv1.Part{
			Start:    durationpb.New(start + sessionOffset),
			End:      durationpb.New(start + sessionOffset + duration),
			Duration: durationpb.New(duration),
		}
	}
	createStint := func(start time.Duration, laps, startLap int32) *predictv1.Part {
		ret := createPart(start, time.Duration(laps)*avgLap)
		ret.PartType = &predictv1.Part_Stint{
			Stint: &predictv1.StintPart{
				Laps:     laps,
				LapStart: startLap,
				LapEnd:   startLap + laps - 1,
			},
		}
		return ret
	}
	createPit := func(start, pitTime time.Duration) *predictv1.Part {
		ret := createPart(start, pitTime)
		ret.PartType = &predictv1.Part_Pit{
			Pit: &predictv1.PitPart{},
		}
		return ret
	}
	// returns duration in race after the pitstop
	addStint := func(start time.Duration, laps, startLap int32) time.Duration {
		p := createStint(start, laps, startLap)
		parts = append(parts, p)
		return start + p.Duration.AsDuration()
	}
	// returns duration in race after the pitstop
	addPitstop := func(start, pitTime time.Duration) time.Duration {
		p := createPit(start, pitTime)
		parts = append(parts, p)
		return start + p.Duration.AsDuration()
	}

	curDur := time.Duration(0) // just init
	// the difficult part: initialization from any possible point in race
	if c.Car.InPit {
		// we need to calc the situation at the end of the current lap
		// curLap += eol.StintLap
	} else {
		// adjust to next reference point only if we compute in a running race
		if c.Car.RemainLapTime != nil && raceStarted {
			if c.Car.RemainLapTime.AsDuration() > 0 {
				curDur = c.Car.RemainLapTime.AsDuration()
			} else {
				curDur = c.Stint.AvgLaptime.AsDuration()
			}
			// if still on first lap, the offset is 0
			// we already have curDur at the end of the first lap
			if c.Race.Lc == 0 {
				sessionOffset = 0
			}
		}
		// check how many laps we can do in this stint
		remain := c.Stint.Lps - c.Car.StintLap
		if remain > 0 {
			// do remaining laps fit all into race?
			if curDur+(time.Duration(remain)*avgLap) < c.Race.Duration.AsDuration() {
				curDur = addStint(curDur, remain, curLap)
				curDur = addPitstop(curDur, pitTime)
				curLap += remain
			} else {
				// calc remaining laps in remaining time and quit
				remain = int32(
					math.Ceil((c.Race.Duration.AsDuration() - curDur).Seconds() / avgLap.Seconds()),
				)
				if remain == 0 {
					return &predictv1.PredictResult{Parts: parts}, nil
				}
				addStint(curDur, remain, curLap)
				return &predictv1.PredictResult{Parts: parts}, nil
			}
		} else {
			// the end of the lap would be the last of that stint
			curDur = addPitstop(curDur, pitTime)

			// special case: the race duration ends while the car is in the pit
			// we need to add another lap
			if curDur >= c.Race.Duration.AsDuration() {
				addStint(curDur, 1, curLap)
				return &predictv1.PredictResult{Parts: parts}, nil
			}
		}
	}

	// the not so difficult part: calc to the end of race

	for curDur < c.Race.Duration.AsDuration() {
		if curDur+stintTime < c.Race.Duration.AsDuration() {
			curDur = addStint(curDur, c.Stint.Lps, curLap)
			curDur = addPitstop(curDur, pitTime)
			curLap += c.Stint.Lps

			// after the pitstop the race is over.
			// nevertheless we need another stint to finish the race
			if curDur >= c.Race.Duration.AsDuration() {
				curDur = addStint(curDur, 1, curLap)
			}
		} else {
			remainDur := c.Race.Duration.AsDuration() - curDur
			laps := int32(math.Ceil(remainDur.Seconds() / avgLap.Seconds()))

			curDur = addStint(curDur, laps, curLap)
		}
	}

	return &predictv1.PredictResult{Parts: parts}, nil
}
