package race

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

type RaceProcessor struct {
	// carNum
	RaceOrder        []string
	CarLaps          map[string]model.AnalysisCarLaps     // key carNum
	RaceGraph        map[string][]model.AnalysisRaceGraph // key: "overall", carClass names
	carProcessor     *car.CarProcessor
	payloadExtractor *util.PayloadExtractor
}

type RaceProcessorOption func(rp *RaceProcessor)

func WithCarProcessor(cp *car.CarProcessor) RaceProcessorOption {
	return func(rp *RaceProcessor) {
		rp.carProcessor = cp
	}
}

func WithPayloadExtractor(pe *util.PayloadExtractor) RaceProcessorOption {
	return func(rp *RaceProcessor) {
		rp.payloadExtractor = pe
	}
}

func NewRaceProcessor(opts ...RaceProcessorOption) *RaceProcessor {
	ret := &RaceProcessor{
		RaceOrder: make([]string, 0),
		RaceGraph: make(map[string][]model.AnalysisRaceGraph),
		CarLaps:   make(map[string]model.AnalysisCarLaps),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// processes the given state message.
// This message must be already processed by the CarProcessor
func (p *RaceProcessor) ProcessStatePayload(payload *model.StatePayload) {
	_, ok := p.payloadExtractor.ExtractSessionValue(
		payload.Session, "sessionTime").(float64)
	if !ok {
		return
	}

	if !p.checkValidData(payload) {
		return
	}

	// race order
	p.RaceOrder = make([]string, 0)
	for i := range payload.Cars {
		carNum := p.carProcessor.NumByIdx[p.getInt(payload.Cars[i], "carIdx")]
		p.RaceOrder = append(p.RaceOrder, carNum)
	}
	// car laps
	p.processCarLaps(payload)
	// race graph
	p.processOverallRaceGraph(payload)
}

func (p *RaceProcessor) processOverallRaceGraph(payload *model.StatePayload) {
	leaderEntry := slices.IndexFunc(payload.Cars,
		func(item []interface{}) bool {
			return p.getInt(item, "pos") == 1
		})
	if leaderEntry == -1 {
		return
	}

	// this will be the new entry for that lap
	raceGraphLapEntry := model.AnalysisRaceGraph{
		LapNo:    p.getInt(payload.Cars[leaderEntry], "lc"),
		CarClass: "overall",
		Gaps:     make([]model.AnalysisGapInfo, 0),
	}
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		gapInfo := model.AnalysisGapInfo{
			CarNum: p.carProcessor.NumByIdx[p.getInt(carMsgEntry, "carIdx")],
			LapNo:  p.getInt(carMsgEntry, "lc"),
			Pos:    p.getInt(carMsgEntry, "pos"),
			Gap:    p.getFloat(carMsgEntry, "gap"),
		}
		if p.payloadExtractor.HasCarKey("pic") {
			gapInfo.Pic = p.getInt(carMsgEntry, "pic")
		}
		raceGraphLapEntry.Gaps = append(raceGraphLapEntry.Gaps, gapInfo)
	}
	slices.SortStableFunc(raceGraphLapEntry.Gaps, func(a, b model.AnalysisGapInfo) int {
		return a.Pos - b.Pos
	})
	// remove existing entry from slice
	lapEntry := slices.IndexFunc(p.RaceGraph["overall"],
		func(item model.AnalysisRaceGraph) bool {
			return item.LapNo == raceGraphLapEntry.LapNo
		})
	if lapEntry != -1 {
		p.RaceGraph["overall"][lapEntry] = raceGraphLapEntry
	} else {
		p.RaceGraph["overall"] = append(p.RaceGraph["overall"], raceGraphLapEntry)
	}
}

func (p *RaceProcessor) processCarLaps(payload *model.StatePayload) {
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		carNum := p.carProcessor.NumByIdx[p.getInt(carMsgEntry, "carIdx")]
		lap := p.getInt(carMsgEntry, "lc")
		laptime := p.getLaptime(carMsgEntry, "last")
		if lap == -1 {
			continue // do not process invalid laps
		}
		carEntry, ok := p.CarLaps[carNum]
		if !ok {
			carEntry = model.AnalysisCarLaps{
				CarNum: carNum,
				Laps:   make([]model.AnalysisLapInfo, 0),
			}
		}
		if idx := slices.IndexFunc(carEntry.Laps,
			func(item model.AnalysisLapInfo) bool { return item.LapNo == lap }); idx != -1 {
			// lap may be updated, so add replace it
			carEntry.Laps[idx] = model.AnalysisLapInfo{
				LapNo:   lap,
				LapTime: laptime,
			}
		} else {
			carEntry.Laps = append(carEntry.Laps, model.AnalysisLapInfo{
				LapNo:   lap,
				LapTime: laptime,
			})
		}
		p.CarLaps[carNum] = carEntry
	}
}

func (p *RaceProcessor) checkValidData(payload *model.StatePayload) bool {
	for i := range payload.Cars {
		carMsgEntry := payload.Cars[i]
		carIdx := p.getInt(carMsgEntry, "carIdx")
		if carIdx == -1 {
			return false
		}
		carNum := p.carProcessor.NumByIdx[carIdx]
		if carNum == "" {
			// silent continue.
			fmt.Printf("CarIdx %d not found\n", carIdx)
			return false
		}
		if !p.checkCarValues(carMsgEntry) {
			// silent continue. Not all values are available
			return false
		}
		p.RaceOrder = append(p.RaceOrder, carNum)
	}
	return true
}

func (p *RaceProcessor) checkCarValues(carMsgEntry []interface{}) bool {
	ret := true
	for _, key := range []string{"pos", "lap", "lc", "gap", "last", "best"} {
		ret = ret && p.payloadExtractor.ExtractCarValue(carMsgEntry, key) != nil
	}
	return ret
}

func (p *RaceProcessor) getInt(carMsgEntry []interface{}, key string) int {
	rawVal := p.payloadExtractor.ExtractCarValue(carMsgEntry, key)
	switch val := rawVal.(type) {
	case int:
		return val
	case float64:
		return int(val)
	default:
		fmt.Printf("Error extracting int val %s: %v %T\n", key, rawVal, rawVal)
		return -1
	}
}

func (p *RaceProcessor) getFloat(carMsgEntry []interface{}, key string) float64 {
	rawVal := p.payloadExtractor.ExtractCarValue(carMsgEntry, key)
	switch val := rawVal.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	return -1
}

func (p *RaceProcessor) getLaptime(carMsgEntry []interface{}, key string) float64 {
	rawVal := p.payloadExtractor.ExtractCarValue(carMsgEntry, key)
	switch val := rawVal.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case []interface{}:
		switch multiVal := val[0].(type) {
		case float64:
			return multiVal
		case int:
			return float64(multiVal)
		}
	}
	return -1
}
