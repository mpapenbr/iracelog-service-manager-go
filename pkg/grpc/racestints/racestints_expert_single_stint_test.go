//nolint:whitespace,lll,funlen,dupl // readability
package racestints

import (
	"reflect"
	"testing"
	"time"
)

func Test_expertStintCalc_Calc_singleStint(t *testing.T) {
	ecp := &ExpertCalcParams{
		RaceDur: 5 * time.Minute,
		LC:      0,
		Lps:     8,
		AvgLap:  60 * time.Second,
	}
	type fields struct {
		param *ExpertCalcParams
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Result
		wantErr bool
	}{
		{"on first lap", fields{
			param: createCopy(ecp,
				withEolParam(WithStintLap(1), WithSessionAtEol(ecp.AvgLap))),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 4, lapStart: 2, lapEnd: 5, stintTime: 4 * ecp.AvgLap},
			},
		}, false},
		{"on second lap", fields{
			param: createCopy(ecp,
				withLC(1),
				withEolParam(WithStintLap(2), WithSessionAtEol(2*ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 3, lapStart: 3, lapEnd: 5, stintTime: 3 * ecp.AvgLap},
			},
		}, false},
		{"on second to last lap", fields{
			param: createCopy(ecp, withLC(3),
				withEolParam(WithStintLap(4), WithSessionAtEol(4*ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{
				// Note: result is based on end of current lap
				&stintPart{laps: 1, lapStart: 5, lapEnd: 5, stintTime: 1 * ecp.AvgLap},
			},
		}, false},
		{"on last lap", fields{
			param: createCopy(ecp, withLC(4),
				withEolParam(WithStintLap(5), WithSessionAtEol(5*ecp.AvgLap)),
			),
		}, &Result{
			Parts: []Part{},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &expertStintCalc{
				param: tt.fields.param,
			}
			got, err := c.Calc()
			if (err != nil) != tt.wantErr {
				t.Errorf("expertStintCalc.Calc() error = %v, wantErr %v", err, tt.wantErr)
				return
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
