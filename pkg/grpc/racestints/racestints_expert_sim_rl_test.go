//nolint:whitespace,lll,funlen,dupl // readability
package racestints

import (
	"reflect"
	"testing"
	"time"
)

func Test_expertStintCalc_Calc_simulated_rl_cases(t *testing.T) {
	ecp := &ExpertCalcParams{
		RaceDur:     8 * time.Hour,
		LC:          0,
		Lps:         41,
		PitTime:     70 * time.Second,
		PitLaneTime: 15 * time.Second,
		AvgLap:      100 * time.Second,
	}
	type fields struct {
		param  *ExpertCalcParams
		parts  []Part
		eolDur eolComp
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Result
		wantErr bool
	}{
		{"on first lap", fields{
			param: createCopy(ecp,
				withEolParam(WithStintLap(1), WithSessionAtEol(ecp.AvgLap)),
			),
			eolDur: func() *EndOfLapData {
				return &EndOfLapData{
					CarInPit: false,
					StintLap: 1,
					// RemainLapTime: ecp.AvgLap,
					SessionAtEol: ecp.AvgLap,
				}
			},
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 40, lapStart: 2, lapEnd: 41, stintTime: 40 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 42, lapEnd: 82, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 83, lapEnd: 123, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 124, lapEnd: 164, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 165, lapEnd: 205, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 206, lapEnd: 246, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 38, lapStart: 247, lapEnd: 284, stintTime: 38 * ecp.AvgLap},
			},
		}, false},
		{"at 93m", fields{
			param: createCopy(ecp,
				withLC(53), withRaceDur(6*time.Hour+29*time.Minute),
				withEolParam(WithStintLap(13), WithSessionAtEol(ecp.AvgLap)),
			),
			eolDur: func() *EndOfLapData {
				return &EndOfLapData{
					CarInPit: false,
					StintLap: 13,
					// RemainLapTime: ecp.AvgLap,
					SessionAtEol: 0,
				}
			},
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 28, lapStart: 55, lapEnd: 82, stintTime: 28 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 83, lapEnd: 123, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 124, lapEnd: 164, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 165, lapEnd: 205, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 41, lapStart: 206, lapEnd: 246, stintTime: 41 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 37, lapStart: 247, lapEnd: 283, stintTime: 37 * ecp.AvgLap},
			},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &expertStintCalc{
				param: tt.fields.param,
				parts: tt.fields.parts,
			}
			got, err := c.Calc()
			if (err != nil) != tt.wantErr {
				t.Errorf("expertStintCalc.Calc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, p := range got.Parts {
				t.Logf("part: %+v\n", p)
			}
			if len(got.Parts) != len(tt.want.Parts) {
				t.Errorf("got %d parts, want %d parts", len(got.Parts), len(tt.want.Parts))
				return
			}
			for i := range got.Parts {
				if !reflect.DeepEqual(got.Parts[i], tt.want.Parts[i]) {
					t.Errorf("part %d: got %v, want %v", i, got.Parts[i], tt.want.Parts[i])
				}
			}
		})
	}
}
