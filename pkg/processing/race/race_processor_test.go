//nolint:thelper,whitespace,lll,funlen,gocritic,dupl,errcheck // ok for tests
package race

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

// to be moved to testsupport
func sampleManifests() *model.Manifests {
	return &model.Manifests{
		Car:     []string{"carIdx", "state", "pos", "pic", "lap", "lc", "gap", "last", "best"},
		Session: []string{"sessionTime", "flagState", "sessionNum"},
		Message: []string{"type", "subType", "msg"},
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
	}
}

func TestRaceProcessor_ExtractValues(t *testing.T) {
	p := NewRaceProcessor(WithPayloadExtractor(util.NewPayloadExtractor(sampleManifests())))
	payload := model.StatePayload{
		//[]string{"carIdx", "state", "pos", "pic", "lap", "lc", "gap", "last", "best"},
		Cars: [][]interface{}{{1, "RUN", 2, 3, 4, 5, 0.5, []interface{}{1.3, "ob"}, 1.7}},
	}

	p.checkCarValues(payload.Cars[0])
	assert.Equal(t, 1, p.getInt(payload.Cars[0], "carIdx"))
	assert.Equal(t, 2, p.getInt(payload.Cars[0], "pos"))
	assert.Equal(t, 3, p.getInt(payload.Cars[0], "pic"))
	assert.Equal(t, 4, p.getInt(payload.Cars[0], "lap"))
	assert.Equal(t, 5, p.getInt(payload.Cars[0], "lc"))
	assert.Equal(t, 1.3, p.getLaptime(payload.Cars[0], "last"))
	assert.Equal(t, 1.7, p.getLaptime(payload.Cars[0], "best"))
}

func TestRaceProcessor_ProcessStatePayload(t *testing.T) {
	cp := car.NewCarProcessor(car.WithManifests(sampleManifests()))

	cp.ProcessCarPayload(sampleCarPayload())
	p := NewRaceProcessor(
		WithCarProcessor(cp),
		WithPayloadExtractor(util.NewPayloadExtractor(sampleManifests())),
	)
	//[]string{"carIdx", "state", "pos", "pic", "lap", "lc", "gap", "last", "best"},
	stateMsgs := []*model.StateData{
		{
			Type: int(model.MTState), Timestamp: 10, Payload: model.StatePayload{
				Session: []interface{}{100.0, "GREEN", 0},
				Cars: [][]interface{}{
					{1, "RUN", 1, 1, 1, 1, 0, -1, 0},
					{2, "RUN", 2, 2, 1, 1, 0, -1, 0},
				},
			},
		},
		{
			Type: int(model.MTState), Timestamp: 20, Payload: model.StatePayload{
				Session: []interface{}{200.0, "GREEN", 0},
				Cars: [][]interface{}{
					{1, "RUN", 1, 1, 2, 1, 0, 14, 0},
					{2, "RUN", 2, 2, 2, 1, 10, -1, 0},
				},
			},
		},
		{
			Type: int(model.MTState), Timestamp: 30, Payload: model.StatePayload{
				Session: []interface{}{300.0, "GREEN", 0},
				Cars: [][]interface{}{
					{1, "RUN", 1, 1, 3, 2, 0, 10.0, 0},
					{2, "RUN", 2, 2, 3, 2, 10, 11.0, 0},
				},
			},
		},
		{
			Type: int(model.MTState), Timestamp: 40, Payload: model.StatePayload{
				Session: []interface{}{400.0, "GREEN", 0},
				Cars: [][]interface{}{
					{1, "RUN", 1, 1, 4, 3, 0, []interface{}{8, "ob"}, 0},
					{2, "RUN", 2, 2, 4, 3, 20, 12, 0},
				},
			},
		},
	}
	for i := range stateMsgs {
		cp.ProcessStatePayload(&stateMsgs[i].Payload)
		p.ProcessStatePayload(stateMsgs[i])
	}

	// check race order
	assert.Equal(t, []string{"10", "20"}, p.RaceOrder)
	// check race graph

	assert.Equal(t, map[string][]model.AnalysisRaceGraph{
		"overall": {
			{LapNo: 1, CarClass: "overall", Gaps: []model.AnalysisGapInfo{
				{CarNum: "10", LapNo: 1, Pos: 1, Gap: 0, Pic: 1},
				{CarNum: "20", LapNo: 1, Pos: 2, Gap: 10, Pic: 2},
			}},
			{LapNo: 2, CarClass: "overall", Gaps: []model.AnalysisGapInfo{
				{CarNum: "10", LapNo: 2, Pos: 1, Gap: 0, Pic: 1},
				{CarNum: "20", LapNo: 2, Pos: 2, Gap: 10, Pic: 2},
			}},
			{LapNo: 3, CarClass: "overall", Gaps: []model.AnalysisGapInfo{
				{CarNum: "10", LapNo: 3, Pos: 1, Gap: 0, Pic: 1},
				{CarNum: "20", LapNo: 3, Pos: 2, Gap: 20, Pic: 2},
			}},
		},
	}, p.RaceGraph)

	// check car laps
	assert.Equal(t, map[string]model.AnalysisCarLaps{
		"10": {CarNum: "10", Laps: []model.AnalysisLapInfo{
			{LapNo: 1, LapTime: 14}, {LapNo: 2, LapTime: 10}, {LapNo: 3, LapTime: 8},
		}},
		"20": {CarNum: "20", Laps: []model.AnalysisLapInfo{
			{LapNo: 1, LapTime: -1}, {LapNo: 2, LapTime: 11.0}, {LapNo: 3, LapTime: 12},
		}},
	}, p.CarLaps)
}

func TestRaceProcessor_UpdateReplayInfo(t *testing.T) {
	cp := car.NewCarProcessor(car.WithManifests(sampleManifests()))

	cp.ProcessCarPayload(sampleCarPayload())
	p := NewRaceProcessor(
		WithCarProcessor(cp),
		WithPayloadExtractor(util.NewPayloadExtractor(sampleManifests())),
		WithRaceSession(1),
	)

	type stepData struct {
		name   string
		input  model.StateData
		expect model.ReplayInfo
	}
	steps := []stepData{
		{
			name: "not race session",
			input: model.StateData{
				Type: int(model.MTState), Timestamp: 10, Payload: model.StatePayload{
					Session:  []interface{}{100.0, "GREEN", 0},
					Cars:     [][]interface{}{},
					Messages: [][]interface{}{},
				},
			},
			expect: model.ReplayInfo{},
		},
		{
			name: "pre race start",
			input: model.StateData{
				Type: int(model.MTState), Timestamp: 20, Payload: model.StatePayload{
					Session:  []interface{}{200.0, "PREP", 1},
					Cars:     [][]interface{}{},
					Messages: [][]interface{}{},
				},
			},
			expect: model.ReplayInfo{
				MinTimestamp:   20,
				MinSessionTime: 200,
				MaxSessionTime: 200,
			},
		},
		{
			name: "still pre race",
			input: model.StateData{
				Type: int(model.MTState), Timestamp: 30, Payload: model.StatePayload{
					Session:  []interface{}{300.0, "PREP", 1},
					Cars:     [][]interface{}{},
					Messages: [][]interface{}{},
				},
			},
			expect: model.ReplayInfo{
				MinTimestamp:   20,
				MinSessionTime: 200,
				MaxSessionTime: 300,
			},
		},
		{
			name: "race start message",
			input: model.StateData{
				Type: int(model.MTState), Timestamp: 40, Payload: model.StatePayload{
					Session: []interface{}{400.0, "GREEN", 1},
					Cars:    [][]interface{}{},
					Messages: [][]interface{}{
						{"Dummy", "Dummy", "Dummy"},
						{"Timing", "RaceControl", "Race start"},
					},
				},
			},
			expect: model.ReplayInfo{
				MinTimestamp:   40,
				MinSessionTime: 400,
				MaxSessionTime: 400,
			},
		},
		{
			name: "after start",
			input: model.StateData{
				Type: int(model.MTState), Timestamp: 50, Payload: model.StatePayload{
					Session:  []interface{}{500.0, "GREEN", 1},
					Cars:     [][]interface{}{},
					Messages: [][]interface{}{},
				},
			},
			expect: model.ReplayInfo{
				MinTimestamp:   40,
				MinSessionTime: 400,
				MaxSessionTime: 500,
			},
		},
	}

	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			p.processReplayInfo(&steps[i].input, step.input.Payload.Session[0].(float64))
			assert.Equal(t, step.expect, p.ReplayInfo)
		})
	}
}
