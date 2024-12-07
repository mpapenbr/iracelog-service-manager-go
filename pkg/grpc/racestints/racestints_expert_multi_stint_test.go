//nolint:whitespace,lll,funlen,dupl // readability
package racestints

import (
	"reflect"
	"testing"
	"time"
)

func Test_expertStintCalc_Calc_multiStint(t *testing.T) {
	ecp := &ExpertCalcParams{
		RaceDur:     10 * time.Minute,
		LC:          0,
		Lps:         8,
		PitTime:     10 * time.Second,
		PitLaneTime: 15 * time.Second,
		AvgLap:      60 * time.Second,
	}
	type fields struct {
		param *ExpertCalcParams
		parts []Part
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
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 7, lapStart: 2, lapEnd: 8, stintTime: 7 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 2, lapStart: 9, lapEnd: 10, stintTime: 2 * ecp.AvgLap},
			},
		}, false},
		{"need 3 stints", fields{
			param: createCopy(ecp,
				withLps(4),
				withEolParam(WithStintLap(1), WithSessionAtEol(ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 3, lapStart: 2, lapEnd: 4, stintTime: 3 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 4, lapStart: 5, lapEnd: 8, stintTime: 4 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 2, lapStart: 9, lapEnd: 10, stintTime: 2 * ecp.AvgLap},
			},
		}, false},
		// car can only do one more lap in stint
		{"one more stint lap", fields{
			param: createCopy(ecp, withLps(4), withLC(6),
				withEolParam(WithStintLap(3), WithSessionAtEol(8*ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 1, lapStart: 8, lapEnd: 8, stintTime: ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 1, lapStart: 9, lapEnd: 9, stintTime: ecp.AvgLap},
			},
		}, false},
		// eol is stint end
		{"on last stint lap", fields{
			param: createCopy(ecp, withLps(4), withLC(7),
				withEolParam(WithStintLap(4), WithSessionAtEol(8*ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 2, lapStart: 9, lapEnd: 10, stintTime: 2 * ecp.AvgLap},
			},
		}, false},

		{"next pit stop overlaps race end", fields{
			param: createCopy(ecp, withLps(4), withLC(8),
				withEolParam(WithStintLap(4), WithSessionAtEol(10*ecp.AvgLap-2*time.Second)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 1, lapStart: 11, lapEnd: 11, stintTime: ecp.AvgLap},
			},
		}, false},

		// Note: LC and lap nums are not important here
		//
		{"last stint ends just before race duration", fields{
			param: createCopy(ecp, withLps(4), withLC(5),
				withEolParam(
					WithStintLap(4),
					// let's fake this to be a last full stint is possible
					// by subtracting 2 seconds and the pit time
					WithSessionAtEol(6*ecp.AvgLap-ecp.PitTime-2*time.Second)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 4, lapStart: 7, lapEnd: 10, stintTime: 4 * ecp.AvgLap},
				&pitPart{pitTime: ecp.PitTime},
				&stintPart{laps: 1, lapStart: 11, lapEnd: 11, stintTime: ecp.AvgLap},
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
