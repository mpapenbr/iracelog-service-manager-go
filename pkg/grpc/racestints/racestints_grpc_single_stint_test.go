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
	pp := &predictv1.PredictParam{
		Race: &predictv1.RaceParam{
			Duration: durationpb.New(conv("5m10s")),
			Lc:       0,
			Session:  durationpb.New(0),
		},
		Stint: &predictv1.StintParam{
			Lps:        8,
			AvgLaptime: durationpb.New(60 * time.Second),
		},
		// Pit not needed with these test
		Car: &predictv1.CarParam{
			CurrentTrackPos: 0,
			InPit:           false,
			StintLap:        0,
			RemainLapTime:   durationpb.New(0),
		},
		Pit: &predictv1.PitParam{
			Overall: durationpb.New(10 * time.Second),
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
			"first lap start", fields{
				param: CopyPredictParam(pp,
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("60s"), conv("5m"),
						createPartStint(5, 2, 6)),
					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			"first lap last seconds", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race, WithRPSession(conv("45s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(conv("15s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("15s"), conv("5m"),
						createPartStint(5, 2, 6)),
					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			"first lap end", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race, WithRPSession(conv("60s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(0)))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("60s"), conv("5m"),
						createPartStint(5, 2, 6)),
					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			// we calc spot on. we don't need to race a little bit longer than race duration
			// that's why we only do additional 4 laps in the 5 minute race
			"race duration is multiple of laptime", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPDuration(conv("5m")), WithRPSession(conv("60s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(0)))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("60s"), conv("4m"),
						createPartStint(4, 2, 5)),
					// Note: result is based on end of current lap

				},
			}, false,
		},
		// Note: to check "near end of race" behavior the important part is the RaceParam
		{
			"second lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPLc(1),
							WithRPDuration(conv("4m10s")),
							WithRPSession(conv("90s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(2),
							WithCPRemainLapTime(conv("30s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("2m"), conv("4m"),
						createPartStint(4, 3, 6)),

					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			"second to last lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPLc(4),
							WithRPDuration(conv("1m10s")),
							WithRPSession(conv("4m30s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(2),
							WithCPRemainLapTime(conv("30s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("5m"), conv("60s"),
						createPartStint(1, 6, 6)),

					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			"last lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPLc(5),
							WithRPDuration(conv("10s")),
							WithRPSession(conv("5m30s")))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(2),
							WithCPRemainLapTime(conv("30s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					// empty by design for last lap
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
