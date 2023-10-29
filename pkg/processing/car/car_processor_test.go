//nolint:thelper,whitespace,lll,funlen,gocritic,dupl // ok for tests
package car

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

// to be moved to testsupport
func sampleManifests() *model.Manifests {
	return &model.Manifests{
		Car:     []string{"carIdx", "state", "lap"},
		Session: []string{"sessionTime", "flagState"},
	}
}

// to be moved to testsupport
func sampleCarPayload() *model.CarPayload {
	// d :=
	return &model.CarPayload{
		Entries: []model.CarEntry{
			{
				Car: model.Car{
					CarIdx:    1,
					CarNumber: "10",
				},
				Drivers: []model.Driver{{CarIdx: 1, Name: "A1"}, {CarIdx: 1, Name: "A2"}},
				Team:    model.Team{CarIdx: 1, Name: "Team A"},
			},
			{
				Car: model.Car{
					CarIdx:    2,
					CarNumber: "20",
				},
				Drivers: []model.Driver{{CarIdx: 2, Name: "B1"}, {CarIdx: 2, Name: "B2"}},
				Team:    model.Team{CarIdx: 2, Name: "Team B"},
			},
		},
		CurrentDrivers: map[int]string{1: "A1", 2: "B1"},
		SessionTime:    100.0,
	}
}

//nolint:thelper,whitespace,lll,funlen // ok for tests
func TestCarProcessor_ProcessCarPayload(t *testing.T) {
	type fields struct {
		init   []CarProcessorOption
		checks func(tt *testing.T, cp *CarProcessor)
	}
	type args struct {
		payload *model.CarPayload
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "first carload message",
			fields: fields{
				init: []CarProcessorOption{},
				checks: func(t *testing.T, cp *CarProcessor) {
					t.Helper()
					// check idx->num mapping
					if !reflect.DeepEqual(cp.NumByIdx, map[int]string{1: "10", 2: "20"}) {
						t.Errorf("NumByIdx not correct")
					}
					// check initial compute states
					if !reflect.DeepEqual(cp.ComputeState,
						map[string]model.AnalysisCarComputeState{
							"10": {CarNum: "10", State: StateInit, OutEncountered: 0.0},
							"20": {CarNum: "20", State: StateInit, OutEncountered: 0.0},
						}) {
						t.Errorf("ComputeState not correct")
					}
				},
			},
			args: args{sampleCarPayload()},
		},
		// TODO: Add test cases.

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewCarProcessor(tt.fields.init...)
			p.ProcessCarPayload(tt.args.payload)
			tt.fields.checks(t, p)
		})
	}
}

func TestCarProcessor_ProcessStatePayload_NoCarData(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))
	// no car data by test design
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
	})
	// check ComputeState
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}
	// check StintLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}

	// check PitLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarPits{}, p.PitLookup); diff != "" {
		t.Errorf("PitLookup not correct: %s", diff)
	}
}

func TestCarProcessor_ProcessStatePayload_StandardRun(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{200.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
	})
	// check ComputeState
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateRun, OutEncountered: 0.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}
	// check StintLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      200.0,
				StintTime:      100.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      200.0,
				StintTime:      100.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}

	// check PitLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarPits{}, p.PitLookup); diff != "" {
		t.Errorf("PitLookup not correct: %s", diff)
	}
}

// Cars perform standard pitstops
func TestCarProcessor_ProcessStatePayload_StandardWithPitstop(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{200.0, "GREEN"},
			Cars:    [][]interface{}{{1, "PIT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{250.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{300.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
	})
	// check ComputeState
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateRun, OutEncountered: 0.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}
	// check StintLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				ExitTime:       250.0,
				EnterTime:      300.0,
				StintTime:      50.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{
				{
					ExitTime:       100.0,
					EnterTime:      200.0,
					StintTime:      100.0,
					LapExit:        1,
					LapEnter:       1,
					NumLaps:        1,
					IsCurrentStint: false,
				},
			}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      300.0,
				StintTime:      200.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}

	// check PitLookup
	if diff := cmp.Diff(
		map[string]model.AnalysisCarPits{
			"10": {CarNum: "10", Current: model.AnalysisPitInfo{}, History: []model.AnalysisPitInfo{
				{
					ExitTime:         250.0,
					EnterTime:        200.0,
					LaneTime:         50.0,
					LapExit:          1,
					LapEnter:         1,
					IsCurrentPitstop: false,
				},
			}},
		}, p.PitLookup); diff != "" {
		t.Errorf("PitLookup not correct: %s", diff)
	}
}

// Car 1 gets OUT but rejoins within threshold
func TestCarProcessor_ProcessStatePayload_Out_Within_Threshold(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{200.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{250.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
	})
	// checks
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateRun, OutEncountered: 0.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      250.0,
				StintTime:      150.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      250.0,
				StintTime:      150.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}
}

// Car 1 is OUT longer than threshold -> stint ends at first OUT state
func TestCarProcessor_ProcessStatePayload_RUN_Out_Exceeds_Threshold(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{200.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{270.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
	})
	// checks
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateOut, OutEncountered: 200.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}

	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				IsCurrentStint: false,
			}, History: []model.AnalysisStintInfo{
				{
					ExitTime:       100.0,
					EnterTime:      270.0,
					StintTime:      170.0,
					LapExit:        1,
					LapEnter:       1,
					NumLaps:        1,
					IsCurrentStint: true,
				},
			}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      270.0,
				StintTime:      170.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}
}

func TestCarProcessor_ProcessStatePayload_PIT_Out_Exceeds_Threshold(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{150.0, "GREEN"},
			Cars:    [][]interface{}{{1, "PIT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{200.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{270.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
	})
	// check state
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateOut, OutEncountered: 200.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}

	// check stint
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				IsCurrentStint: false,
			}, History: []model.AnalysisStintInfo{
				{
					ExitTime:       100.0,
					EnterTime:      150.0,
					StintTime:      50.0,
					LapExit:        1,
					LapEnter:       1,
					NumLaps:        1,
					IsCurrentStint: false,
				},
			}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      270.0,
				StintTime:      170.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}
	// check pit
	if diff := cmp.Diff(
		map[string]model.AnalysisCarPits{
			"10": {CarNum: "10", Current: model.AnalysisPitInfo{
				ExitTime:         270.0,
				EnterTime:        150.0,
				LaneTime:         120.0,
				LapExit:          1,
				LapEnter:         1,
				IsCurrentPitstop: true,
			}, History: []model.AnalysisPitInfo{}},
		}, p.PitLookup); diff != "" {
		t.Errorf("PitLookup not correct: %s", diff)
	}
}

func TestCarProcessor_ProcessStatePayload_Car_Rejoins_After_being_OUT(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))

	p.ProcessCarPayload(sampleCarPayload())
	p.ProcessStatePayloads([]*model.StatePayload{
		{
			Session: []interface{}{100.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{150.0, "GREEN"},
			Cars:    [][]interface{}{{1, "PIT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{250.0, "GREEN"},
			Cars:    [][]interface{}{{1, "OUT", 1}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{270.0, "GREEN"},
			Cars:    [][]interface{}{{1, "PIT", 2}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{280.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 2}, {2, "RUN", 1}},
		},
		{
			Session: []interface{}{290.0, "GREEN"},
			Cars:    [][]interface{}{{1, "RUN", 2}, {2, "RUN", 1}},
		},
	})
	// check state
	if diff := cmp.Diff(
		map[string]model.AnalysisCarComputeState{
			"10": {CarNum: "10", State: StateRun, OutEncountered: 0.0},
			"20": {CarNum: "20", State: StateRun, OutEncountered: 0.0},
		}, p.ComputeState); diff != "" {
		t.Errorf("ComputeState not correct: %s", diff)
	}

	// check stint
	if diff := cmp.Diff(
		map[string]model.AnalysisCarStints{
			"10": {CarNum: "10", Current: model.AnalysisStintInfo{
				ExitTime:       280.0,
				EnterTime:      290.0,
				StintTime:      10.0,
				LapExit:        2,
				LapEnter:       2,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{
				{
					ExitTime:       100.0,
					EnterTime:      150.0,
					StintTime:      50.0,
					LapExit:        1,
					LapEnter:       1,
					NumLaps:        1,
					IsCurrentStint: false,
				},
			}},
			"20": {CarNum: "20", Current: model.AnalysisStintInfo{
				ExitTime:       100.0,
				EnterTime:      290.0,
				StintTime:      190.0,
				LapExit:        1,
				LapEnter:       1,
				NumLaps:        1,
				IsCurrentStint: true,
			}, History: []model.AnalysisStintInfo{}},
		}, p.StintLookup); diff != "" {
		t.Errorf("StintLookup not correct: %s", diff)
	}
	// check pit
	if diff := cmp.Diff(
		map[string]model.AnalysisCarPits{
			"10": {CarNum: "10", Current: model.AnalysisPitInfo{}, History: []model.AnalysisPitInfo{
				{
					ExitTime:         280.0,
					EnterTime:        150.0,
					LaneTime:         130.0,
					LapExit:          2,
					LapEnter:         1,
					IsCurrentPitstop: false,
				},
			}},
		}, p.PitLookup); diff != "" {
		t.Errorf("PitLookup not correct: %s", diff)
	}
}

func TestCarProcessor_ProcessCarPayload_SeatTime(t *testing.T) {
	p := NewCarProcessor(WithManifests(sampleManifests()))
	// p.ProcessCarPayload(sampleCarPayload())
	type args struct {
		currentDrivers map[int]string
		sessionTime    float64
	}

	for _, item := range []args{
		{sessionTime: 100, currentDrivers: map[int]string{1: "A1", 2: "B1"}},
		{sessionTime: 200, currentDrivers: map[int]string{1: "A1", 2: "B2"}},
		{sessionTime: 300, currentDrivers: map[int]string{1: "A2", 2: "B2"}},
		{sessionTime: 400, currentDrivers: map[int]string{1: "A1", 2: "B2"}},
		{sessionTime: 500, currentDrivers: map[int]string{1: "A1", 2: "B1"}},
	} {
		payload := sampleCarPayload()
		payload.CurrentDrivers = item.currentDrivers
		payload.SessionTime = item.sessionTime
		p.ProcessCarPayload(payload)
	}
	assert.Equal(t, map[string]model.AnalysisCarInfo{
		"10": {
			CarNum: "10", Drivers: []model.AnalysisDriverInfo{
				{DriverName: "A1", SeatTime: []model.AnalysisSeatTime{
					{EnterCarTime: 100, LeaveCarTime: 300},
					{EnterCarTime: 400, LeaveCarTime: 500},
				}},
				{DriverName: "A2", SeatTime: []model.AnalysisSeatTime{
					{EnterCarTime: 300, LeaveCarTime: 400},
				}},
			},
		},
		"20": {
			CarNum: "20", Drivers: []model.AnalysisDriverInfo{
				{DriverName: "B1", SeatTime: []model.AnalysisSeatTime{
					{EnterCarTime: 100, LeaveCarTime: 200},
					{EnterCarTime: 500, LeaveCarTime: 500},
				}},
				{DriverName: "B2", SeatTime: []model.AnalysisSeatTime{
					{EnterCarTime: 200, LeaveCarTime: 500},
				}},
			},
		},
	}, p.CarInfoLookup)
}
