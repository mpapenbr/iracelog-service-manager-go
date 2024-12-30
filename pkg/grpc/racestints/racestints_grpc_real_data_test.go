//nolint:whitespace,lll,funlen,dupl // readability

// These are no real tests, just some real data to check the output of the Calc function
package racestints

import (
	"fmt"
	"testing"

	predictv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/predict/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func clearNanos(t *durationpb.Duration) *durationpb.Duration {
	return &durationpb.Duration{Seconds: t.GetSeconds(), Nanos: 0}
}

func formattedParts(parts []*predictv1.Part) string {
	common := func(p *predictv1.Part) string {
		return fmt.Sprintf("Start: %-9s End: %-9s Dur: %-8s",
			clearNanos(p.Start).AsDuration(),
			clearNanos(p.End).AsDuration(),
			clearNanos(p.Duration).AsDuration())
	}
	ret := "\n"
	for _, p := range parts {
		switch p.PartType.(type) {
		case *predictv1.Part_Pit:
			ret += fmt.Sprintf("Pit  : %s\n", common(p))
		case *predictv1.Part_Stint:
			stint := p.GetStint()
			ret += fmt.Sprintf("Stint: %s %3d-%3d (%d)\n",
				common(p), stint.LapStart, stint.LapEnd, stint.Laps)
		default:
			fmt.Println("Unknown part type")
		}
	}
	return ret
}

//nolint:dupl // by design
func Test_real_data_fuji8h_leader(t *testing.T) {
	ppLeader := &predictv1.PredictParam{
		Race: &predictv1.RaceParam{
			Duration: durationpb.New(conv("14556.750s")),
			// Duration: durationpb.New(conv("4h")),
			Lc:      140,
			Session: durationpb.New(conv("14460.20s")),
			// Session: durationpb.New(conv("0s")),
		},
		Stint: &predictv1.StintParam{
			Lps:        41,
			AvgLaptime: durationpb.New(conv("99.854s")),
		},

		Car: &predictv1.CarParam{
			CurrentTrackPos: 0,
			InPit:           false,
			StintLap:        18,
			RemainLapTime:   durationpb.New(conv("30.926s")),
		},
		Pit: &predictv1.PitParam{
			Overall: durationpb.New(conv("70.6s")),
		},
	}
	got, err := Calc(ppLeader)
	if err != nil {
		t.Errorf("Calc() error = %v", err)
		return
	}
	t.Logf("%s\n", formattedParts(got.Parts))
}

//nolint:dupl // by design
func Test_real_data_fuji8h_002(t *testing.T) {
	ppCar002 := &predictv1.PredictParam{
		Race: &predictv1.RaceParam{
			Duration: durationpb.New(conv("14556.750s")),
			// Duration: durationpb.New(conv("4h12s")),
			Lc: 140,
			// Session:  durationpb.New(conv("2h")),
			Session: durationpb.New(conv("14460.20s")),
		},
		Stint: &predictv1.StintParam{
			Lps:        40,
			AvgLaptime: durationpb.New(conv("99.795s")),
			// AvgLaptime: durationpb.New(conv("100s")),
		},

		Car: &predictv1.CarParam{
			CurrentTrackPos: 0,
			InPit:           false,
			StintLap:        21,
			RemainLapTime:   durationpb.New(conv("14.831s")),
		},
		Pit: &predictv1.PitParam{
			Overall: durationpb.New(conv("72.6s")),
		},
	}
	got, err := Calc(ppCar002)
	if err != nil {
		t.Errorf("Calc() error = %v", err)
		return
	}
	t.Logf("%s\n", formattedParts(got.Parts))
}
