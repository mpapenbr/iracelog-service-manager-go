package livedata

import (
	"reflect"
	"testing"

	analysisv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/analysis/v1"
)

func Test_tailedCarlaps(t *testing.T) {
	type args struct {
		in   []*analysisv1.CarLaps
		tail uint32
	}
	tests := []struct {
		name string
		args args
		want []*analysisv1.CarLaps
	}{
		{name: "empty", args: args{in: []*analysisv1.CarLaps{}, tail: 0}, want: []*analysisv1.CarLaps{}},
		{name: "last when empty", args: args{in: []*analysisv1.CarLaps{}, tail: 1}, want: []*analysisv1.CarLaps{}},
		{
			name: "last on single",
			args: args{
				in: []*analysisv1.CarLaps{
					{CarNum: "1", Laps: []*analysisv1.Lap{{LapNo: 1, LapTime: 1}}},
				}, tail: 1,
			},
			want: []*analysisv1.CarLaps{
				{CarNum: "1", Laps: []*analysisv1.Lap{{LapNo: 1, LapTime: 1}}},
			},
		},
		{
			name: "last on single",
			args: args{
				in: []*analysisv1.CarLaps{
					{
						CarNum: "1", Laps: []*analysisv1.Lap{
							{LapNo: 1, LapTime: 1},
							{LapNo: 2, LapTime: 2},
						},
					},
				},
				tail: 1,
			},
			want: []*analysisv1.CarLaps{
				{CarNum: "1", Laps: []*analysisv1.Lap{{LapNo: 2, LapTime: 2}}},
			},
		},
		{
			name: "tail exceeds length",
			args: args{
				in: []*analysisv1.CarLaps{
					{
						CarNum: "1", Laps: []*analysisv1.Lap{
							{LapNo: 1, LapTime: 1},
						},
					},
				},
				tail: 2,
			},
			want: []*analysisv1.CarLaps{
				{CarNum: "1", Laps: []*analysisv1.Lap{{LapNo: 1, LapTime: 1}}},
			},
		},
		{
			name: "multiple cars",
			args: args{
				in: []*analysisv1.CarLaps{
					{
						CarNum: "1", Laps: []*analysisv1.Lap{
							{LapNo: 1, LapTime: 1},
							{LapNo: 2, LapTime: 2},
						},
					},
					{
						CarNum: "2", Laps: []*analysisv1.Lap{
							{LapNo: 1, LapTime: 3},
							{LapNo: 2, LapTime: 4},
						},
					},
				},
				tail: 1,
			},
			want: []*analysisv1.CarLaps{
				{CarNum: "1", Laps: []*analysisv1.Lap{{LapNo: 2, LapTime: 2}}},
				{CarNum: "2", Laps: []*analysisv1.Lap{{LapNo: 2, LapTime: 4}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tailedCarlaps(tt.args.in, tt.args.tail); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tailedCarlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tailedRaceGraph(t *testing.T) {
	type args struct {
		in   []*analysisv1.RaceGraph
		tail uint32
	}
	tests := []struct {
		name string
		args args
		want []*analysisv1.RaceGraph
	}{
		{
			name: "empty",
			args: args{
				in:   []*analysisv1.RaceGraph{},
				tail: 0,
			},
			want: []*analysisv1.RaceGraph{},
		},
		{
			name: "request 1 on empty",
			args: args{
				in:   []*analysisv1.RaceGraph{},
				tail: 1,
			},
			want: []*analysisv1.RaceGraph{},
		},
		{
			name: "request last",
			args: args{
				in: []*analysisv1.RaceGraph{
					{
						LapNo:    1,
						CarClass: "A",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "1", Gap: 0},
							{CarNum: "2", Gap: 1},
						},
					},
					{
						LapNo:    1,
						CarClass: "B",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "10", Gap: 0},
							{CarNum: "20", Gap: 1},
						},
					},
					{
						LapNo:    1,
						CarClass: "All",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "1", Gap: 0},
							{CarNum: "2", Gap: 1},
							{CarNum: "10", Gap: 0},
							{CarNum: "20", Gap: 1},
						},
					},
				},
				tail: 1,
			},
			want: []*analysisv1.RaceGraph{
				{
					LapNo:    1,
					CarClass: "A",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "1", Gap: 0},
						{CarNum: "2", Gap: 1},
					},
				},
				{
					LapNo:    1,
					CarClass: "B",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "10", Gap: 0},
						{CarNum: "20", Gap: 1},
					},
				},
				{
					LapNo:    1,
					CarClass: "All",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "1", Gap: 0},
						{CarNum: "2", Gap: 1},
						{CarNum: "10", Gap: 0},
						{CarNum: "20", Gap: 1},
					},
				},
			},
		},
		{
			name: "request last from mulitple laps",
			args: args{
				in: []*analysisv1.RaceGraph{
					{
						LapNo:    1,
						CarClass: "A",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "1", Gap: 0},
							{CarNum: "2", Gap: 1},
						},
					},
					{
						LapNo:    1,
						CarClass: "B",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "10", Gap: 0},
							{CarNum: "20", Gap: 1},
						},
					},
					{
						LapNo:    1,
						CarClass: "C",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "30", Gap: 0},
							{CarNum: "40", Gap: 1},
						},
					},
					{
						LapNo:    2,
						CarClass: "A",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "1", Gap: 0},
							{CarNum: "2", Gap: 10},
						},
					},
					{
						LapNo:    2,
						CarClass: "B",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "10", Gap: 0},
							{CarNum: "20", Gap: 11},
						},
					},
					{
						LapNo:    2,
						CarClass: "C",
						Gaps: []*analysisv1.GapInfo{
							{CarNum: "30", Gap: 0},
							{CarNum: "40", Gap: 11},
						},
					},
				},
				tail: 1,
			},
			want: []*analysisv1.RaceGraph{
				{
					LapNo:    2,
					CarClass: "A",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "1", Gap: 0},
						{CarNum: "2", Gap: 10},
					},
				},
				{
					LapNo:    2,
					CarClass: "B",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "10", Gap: 0},
						{CarNum: "20", Gap: 11},
					},
				},
				{
					LapNo:    2,
					CarClass: "C",
					Gaps: []*analysisv1.GapInfo{
						{CarNum: "30", Gap: 0},
						{CarNum: "40", Gap: 11},
					},
				},
			},
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tailedRaceGraph(tt.args.in, tt.args.tail); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tailedRaceGraph() = %v, want %v", got, tt.want)
			}
		})
	}
}
