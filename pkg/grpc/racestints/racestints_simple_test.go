//nolint:whitespace,lll,funlen // readability
package racestints

import (
	"reflect"
	"testing"
	"time"
)

func Test_simpleStintCalc_Calc(t *testing.T) {
	type fields struct {
		param *SimpleCalcParams
	}
	// converts sec to time.Duration
	toDur := func(secs int) time.Duration { return time.Duration(secs) * time.Second }
	tests := []struct {
		name    string
		fields  fields
		want    *Result
		wantErr bool
	}{
		{
			name: "trivial",
			fields: fields{param: &SimpleCalcParams{
				RaceDur: toDur(1), Lps: 1, PitTime: toDur(1), AvgLap: toDur(1),
			}},
			want: &Result{
				Parts: []Part{&stintPart{laps: 1, lapStart: 1, lapEnd: 1, stintTime: toDur(1)}},
			},
			wantErr: false,
		},
		{
			name: "single stint",
			fields: fields{param: &SimpleCalcParams{
				RaceDur: toDur(10), Lps: 3, PitTime: toDur(5), AvgLap: toDur(4),
			}},
			want: &Result{
				Parts: []Part{&stintPart{laps: 3, lapStart: 1, lapEnd: 3, stintTime: toDur(12)}},
			},
			wantErr: false,
		},
		{
			name: "two stints",
			fields: fields{param: &SimpleCalcParams{
				RaceDur: toDur(20), Lps: 3, PitTime: toDur(5), AvgLap: toDur(4),
			}},
			want: &Result{
				Parts: []Part{
					&stintPart{laps: 3, lapStart: 1, lapEnd: 3, stintTime: toDur(12)},
					&pitPart{pitTime: toDur(5)},
					&stintPart{laps: 1, lapStart: 4, lapEnd: 4, stintTime: toDur(4)},
				},
			},
			wantErr: false,
		},
		{
			name: "pit close to race end",
			// entering pit close to end of race
			// reduce race to 16 to see if another stint is added
			fields: fields{param: &SimpleCalcParams{
				RaceDur: toDur(16), Lps: 3, PitTime: toDur(5), AvgLap: toDur(4),
			}},
			want: &Result{
				Parts: []Part{
					&stintPart{laps: 3, lapStart: 1, lapEnd: 3, stintTime: toDur(12)},
					&pitPart{pitTime: toDur(5)},
					&stintPart{laps: 1, lapStart: 4, lapEnd: 4, stintTime: toDur(4)},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &simpleStintCalc{
				param: tt.fields.param,
			}
			got, err := c.Calc()
			if (err != nil) != tt.wantErr {
				t.Errorf("simpleStintCalc.Calc() error = %v, wantErr %v", err, tt.wantErr)
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
