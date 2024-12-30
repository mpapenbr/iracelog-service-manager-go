//nolint:whitespace,lll,funlen,dupl // readability
package racestints

import (
	"reflect"
	"testing"
	"time"

	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func Test_racestintsCalc_Calc_multiStint(t *testing.T) {
	pitTime := 10 * time.Second
	pp := &predictv1.PredictParam{
		Race: &predictv1.RaceParam{
			Duration: durationpb.New(conv("10m")),
			Lc:       0,
			Session:  durationpb.New(0),
		},
		Stint: &predictv1.StintParam{
			Lps:        8,
			AvgLaptime: durationpb.New(60 * time.Second),
		},

		Car: &predictv1.CarParam{
			CurrentTrackPos: 0,
			InPit:           false,
			StintLap:        0,
			RemainLapTime:   durationpb.New(0),
		},
		Pit: &predictv1.PitParam{
			Overall: durationpb.New(pitTime),
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
			"first lap", fields{
				param: CopyPredictParam(pp,
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("60s"), conv("7m"), createPartStint(7, 2, 8)),
					CreatePitPart(conv("8m"), pitTime, createPartPit()),
					CreateStintPart(conv("8m10s"), conv("2m"), createPartStint(2, 9, 10)),
					// Note: result is based on end of current lap

				},
			}, false,
		},
		{
			"need 3 stints", fields{
				param: CopyPredictParam(pp,
					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(1),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("60s"), conv("3m"), createPartStint(3, 2, 4)),
					CreatePitPart(conv("4m"), pitTime, createPartPit()),
					CreateStintPart(conv("4m10s"), conv("4m"), createPartStint(4, 5, 8)),
					CreatePitPart(conv("8m10s"), pitTime, createPartPit()),
					CreateStintPart(conv("8m20s"), conv("2m"), createPartStint(2, 9, 10)),
					// Note: result is based on end of current lap
				},
			}, false,
		},
		{
			// car can only do one more lap in stint
			// Hint:
			// calc starts at 4m with 1m30s remaining.
			// car enters pit for 10s
			// needs to do 1 more lap to finish the race

			"one more stint lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPSession(conv("3m")),
							WithRPDuration(conv("2m30s")),
							WithRPLc(3),
						),
					),

					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(3),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("4m"), conv("60s"), createPartStint(1, 5, 5)),
					CreatePitPart(conv("5m"), pitTime, createPartPit()),
					CreateStintPart(conv("5m10s"), conv("60s"), createPartStint(1, 6, 6)),

					// Note: result is based on end of current lap
				},
			}, false,
		},
		{
			// car has to pit and do another 2 laps (at start of lap)
			"on last stint lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPSession(conv("3m")),
							WithRPDuration(conv("2m30s")),
							WithRPLc(3),
						),
					),

					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(4),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreatePitPart(conv("4m"), pitTime, createPartPit()),
					CreateStintPart(conv("4m10s"), conv("2m"), createPartStint(2, 5, 6)),

					// Note: result is based on end of current lap
				},
			}, false,
		},
		{
			// car has to pit and do another lap
			// -> in 10s we do a pit stop and add another lap to finish the race
			"on last stint lap with 10s remain lap", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPSession(conv("3m")),
							WithRPDuration(conv("1m10s")),
							WithRPLc(3),
						),
					),

					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(4),
							WithCPRemainLapTime(conv("10s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreatePitPart(conv("3m10s"), pitTime, createPartPit()),
					CreateStintPart(conv("3m20s"), conv("1m"), createPartStint(1, 5, 5)),

					// Note: result is based on end of current lap
				},
			}, false,
		},
		{
			// car is on last stint lap and has to pit with 5s remaining
			// being in the pits the race ends
			// -> needs to do 1 more lap
			"next pit stop overlaps race end", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPSession(conv("3m")),
							WithRPDuration(conv("1m5s")),
							WithRPLc(3),
						),
					),

					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(4),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreatePitPart(conv("4m"), pitTime, createPartPit()),
					CreateStintPart(conv("4m10s"), conv("60s"), createPartStint(1, 5, 5)),

					// Note: result is based on end of current lap
				},
			}, false,
		},
		{
			// car is on last stint lap and has to pit with 5s remaining
			// being in the pits the race ends
			// -> needs to do 1 more lap
			"last stint ends just before race end", fields{
				param: CopyPredictParam(pp,
					WithPPRace(
						CopyRaceParam(pp.Race,
							WithRPSession(conv("3m")),
							WithRPDuration(conv("6m15s")),
							WithRPLc(3),
						),
					),

					WithPPStint(CopyStintParam(pp.Stint, WithSPLps(4))),
					WithPPCar(
						CopyCarParam(pp.Car,
							WithCPStintLap(3),
							WithCPRemainLapTime(conv("60s"))))),
			}, &predictv1.PredictResult{
				Parts: []*predictv1.Part{
					CreateStintPart(conv("4m"), conv("60s"), createPartStint(1, 5, 5)),
					CreatePitPart(conv("5m"), pitTime, createPartPit()),
					CreateStintPart(conv("5m10s"), conv("4m"), createPartStint(4, 6, 9)),
					CreatePitPart(conv("9m10s"), pitTime, createPartPit()),
					CreateStintPart(conv("9m20s"), conv("60s"), createPartStint(1, 10, 10)),

					// Note: result is based on end of current lap
				},
			}, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calc(tt.fields.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("grpcStintCalc.Calc() error = %v, wantErr %v", err, tt.wantErr)
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
