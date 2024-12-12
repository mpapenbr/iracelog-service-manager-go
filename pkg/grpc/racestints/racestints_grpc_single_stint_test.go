//nolint:whitespace,lll,funlen,dupl // readability
package racestints

import (
	"reflect"
	"testing"
	"time"

	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func Test_racestintsCalc_Calc_singleStint(t *testing.T) {
	toDurPb := func(d time.Duration) *durationpb.Duration {
		return durationpb.New(d)
	}

	pp := &predictv1.PredictParam{
		Race: &predictv1.RaceParam{
			Duration: toDurPb(5 * time.Minute),
			Lc:       0,
			Session:  toDurPb(0),
		},
		Stint: &predictv1.StintParam{
			Lps:        8,
			AvgLaptime: toDurPb(60 * time.Second),
		},
		// Pit not needed with these test
		Car: &predictv1.CarParam{
			CurrentTrackPos: 0,
			InPit:           false,
			StintLap:        0,
			RemainLapTime:   toDurPb(0),
		},
		Pit: &predictv1.PitParam{
			Overall: toDurPb(10 * time.Second),
		},
	}
	type fields struct {
		param *predictv1.PredictParam
	}
	tests := []struct {
		name    string
		fields  fields
		want    *predictv1.PredictResult
		wantErr bool
	}{
		{
			"on first lap", fields{
				param: CopyPredictParam(pp, WithPPCar(
					CopyCarParam(pp.Car,
						WithCPStintLap(1),
						WithCPRemainLapTime(pp.Stint.AvgLaptime.AsDuration())))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(60*time.Second, 4*60*time.Second,
						&predictv1.Part_Stint{Stint: &predictv1.StintPart{Laps: 4, LapStart: 2, LapEnd: 5}}),
					// Note: result is based on end of current lap
					// &stintPart{laps: 4, lapStart: 2, lapEnd: 5, stintTime: 4 * pp.AvgLap},
				},
			}, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calc(tt.fields.param)
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
