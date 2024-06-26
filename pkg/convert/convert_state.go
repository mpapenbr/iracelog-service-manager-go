package convert

import (
	"fmt"
	"time"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
)

func NewStateMessageConverter(e *model.DbEvent, eventKey string) *StateConverter {
	ret := &StateConverter{
		payloadExtractor: util.NewPayloadExtractor(&e.Data.Manifests),
		eventKey:         eventKey,
		numSectors:       len(e.Data.Info.Sectors),
	}
	return ret
}

type StateConverter struct {
	payloadExtractor *util.PayloadExtractor
	eventKey         string
	numSectors       int
}

//nolint:lll // better readability
func (p *StateConverter) ConvertStatePayload(m *model.StateData) *racestatev1.PublishStateRequest {
	ret := &racestatev1.PublishStateRequest{
		Event:     &commonv1.EventSelector{Arg: &commonv1.EventSelector_Key{Key: p.eventKey}},
		Cars:      p.convertCars(m.Payload.Cars),
		Session:   p.convertSession(m.Payload.Session),
		Messages:  p.convertMessages(m.Payload.Messages),
		Timestamp: timestamppb.New(time.UnixMilli(int64(m.Timestamp * 1000))),
	}
	return ret
}

func (p *StateConverter) convertCars(cars [][]interface{}) []*racestatev1.Car {
	ret := make([]*racestatev1.Car, 0)
	for _, car := range cars {
		ret = append(ret, p.convertCar(car))
	}
	return ret
}

func (p *StateConverter) convertCar(car []interface{}) *racestatev1.Car {
	extr := p.payloadExtractor.ExtractCarValue
	sectors := make([]*racestatev1.TimeWithMarker, p.numSectors)
	for i := 0; i < p.numSectors; i++ {
		sectors[i] = convertLaptime(car, fmt.Sprintf("s%d", i+1), extr)
	}

	return &racestatev1.Car{
		CarIdx:       int32(p.getIntFrom(car, "carIdx", extr)),
		State:        convertCarState(car, "state", extr),
		Pos:          int32(p.getIntFrom(car, "pos", extr)),
		Pic:          int32(p.getIntFrom(car, "pic", extr)),
		Lap:          int32(p.getIntFrom(car, "lap", extr)),
		Lc:           int32(p.getIntFrom(car, "lc", extr)),
		Pitstops:     uint32(p.getIntFrom(car, "pitstops", extr)),
		StintLap:     uint32(p.getIntFrom(car, "stintLap", extr)),
		TrackPos:     float32(p.getFloatFrom(car, "trackPos", extr)),
		Speed:        float32(p.getFloatFrom(car, "speed", extr)),
		Dist:         float32(p.getFloatFrom(car, "dist", extr)),
		Interval:     float32(p.getFloatFrom(car, "interval", extr)),
		Gap:          float32(p.getFloatFrom(car, "gap", extr)),
		TireCompound: convertTireCompound(car, "tireCompound", extr),
		Best:         convertLaptime(car, "best", extr),
		Last:         convertLaptime(car, "last", extr),
		Sectors:      sectors,
	}
}

func (p *StateConverter) convertSession(session []interface{}) *racestatev1.Session {
	extr := p.payloadExtractor.ExtractSessionValue
	return &racestatev1.Session{
		SessionNum:      uint32(p.getIntFrom(session, "num", extr)),
		SessionTime:     float32(p.getFloatFrom(session, "sessionTime", extr)),
		TrackTemp:       float32(p.getFloatFrom(session, "trackTemp", extr)),
		AirTemp:         float32(p.getFloatFrom(session, "airTemp", extr)),
		AirDensity:      float32(p.getFloatFrom(session, "airDensity", extr)),
		AirPressure:     float32(p.getFloatFrom(session, "airPressure", extr)),
		WindDir:         float32(p.getFloatFrom(session, "windDir", extr)),
		WindVel:         float32(p.getFloatFrom(session, "windVel", extr)),
		TimeOfDay:       uint32(p.getIntFrom(session, "timeOfDay", extr)),
		TimeRemain:      float32(p.getFloatFrom(session, "timeRemain", extr)),
		LapsRemain:      int32(p.getIntFrom(session, "lapsRemain", extr)),
		FlagState:       p.getStringFrom(session, "flagState", extr),
		SessionStateRaw: int32(p.getIntFrom(session, "sessionStateRaw", extr)),
		SessionFlagsRaw: uint32(p.getIntFrom(session, "sessionFlagsRaw", extr)),
		Precipitation:   float32(p.getFloatFrom(session, "precipitation", extr)),
		TrackWetness:    convertTrackWetness(session, "trackWetness", extr),
	}
}

//nolint:lll // better readability
func (p *StateConverter) convertMessages(messages [][]interface{}) []*racestatev1.Message {
	ret := make([]*racestatev1.Message, 0)
	for _, msg := range messages {
		ret = append(ret, p.convertMessage(msg))
	}
	return ret
}

func (p *StateConverter) convertMessage(messages []interface{}) *racestatev1.Message {
	extr := p.payloadExtractor.ExtractMessageValue
	ret := &racestatev1.Message{
		Type:    convertMessageType(messages, "type", extr),
		SubType: convertMessageSubType(messages, "type", extr),
		Msg:     p.getStringFrom(messages, "msg", extr),
	}
	if extr(messages, "carIdx") != nil {
		ret.CarIdx = uint32(p.getIntFrom(messages, "carIdx", extr))
	}
	if extr(messages, "carNum") != nil {
		ret.CarNum = p.getStringFrom(messages, "carNum", extr)
	}
	if extr(messages, "carClass") != nil {
		ret.CarClass = p.getStringFrom(messages, "carClass", extr)
	}

	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func convertLaptime(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) *racestatev1.TimeWithMarker {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case float64:
		return &racestatev1.TimeWithMarker{
			Time:   float32(val),
			Marker: racestatev1.TimeMarker_TIME_MARKER_UNSPECIFIED,
		}
	case int:
		return &racestatev1.TimeWithMarker{
			Time:   float32(val),
			Marker: racestatev1.TimeMarker_TIME_MARKER_UNSPECIFIED,
		}
	case []interface{}:
		switch multiVal := val[0].(type) {
		case float64:
			return &racestatev1.TimeWithMarker{
				Time:   float32(multiVal),
				Marker: convertTimeMarker(val[1].(string)),
			}
		case int:
			return &racestatev1.TimeWithMarker{
				Time:   float32(multiVal),
				Marker: convertTimeMarker(val[1].(string)),
			}
		}
	}
	return &racestatev1.TimeWithMarker{
		Time:   float32(-1),
		Marker: racestatev1.TimeMarker_TIME_MARKER_UNSPECIFIED,
	}
}

func convertTimeMarker(rawVal string) racestatev1.TimeMarker {
	switch rawVal {
	case "ob":
		return racestatev1.TimeMarker_TIME_MARKER_OVERALL_BEST
	case "pb":
		return racestatev1.TimeMarker_TIME_MARKER_PERSONAL_BEST
	case "clb":
		return racestatev1.TimeMarker_TIME_MARKER_CLASS_BEST
	case "cb":
		return racestatev1.TimeMarker_TIME_MARKER_CAR_BEST
	case "old":
		return racestatev1.TimeMarker_TIME_MARKER_OLD_VALUE
	default:
		return racestatev1.TimeMarker_TIME_MARKER_UNSPECIFIED
	}
}

//nolint:whitespace // can't make both editor and linter happy
func convertTireCompound(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) *racestatev1.TireCompound {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case int:
		return &racestatev1.TireCompound{RawValue: uint32(val)}
	case float64:
		return &racestatev1.TireCompound{RawValue: uint32(val)}
	default:
		return nil
	}
}

//nolint:whitespace,cyclop // can't make both editor and linter happy
func convertTrackWetness(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) commonv1.TrackWetness {
	rawVal := extr(msg, key)
	var useVal int
	switch val := rawVal.(type) {
	case int:
		useVal = val
	case float64:
		useVal = int(val)
	}
	switch useVal {
	case 0:
		return commonv1.TrackWetness_TRACK_WETNESS_UNSPECIFIED
	case 1:
		return commonv1.TrackWetness_TRACK_WETNESS_DRY
	case 2:
		return commonv1.TrackWetness_TRACK_WETNESS_MOSTLY_DRY
	case 3:
		return commonv1.TrackWetness_TRACK_WETNESS_VERY_LIGHTLY_WET
	case 4:
		return commonv1.TrackWetness_TRACK_WETNESS_LIGHTLY_WET
	case 5:
		return commonv1.TrackWetness_TRACK_WETNESS_MODERATELY_WET
	case 6:
		return commonv1.TrackWetness_TRACK_WETNESS_VERY_WET
	case 7:
		return commonv1.TrackWetness_TRACK_WETNESS_EXTREMELY_WET

	}
	return commonv1.TrackWetness_TRACK_WETNESS_UNSPECIFIED
}

//nolint:whitespace // can't make both editor and linter happy
func convertCarState(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) racestatev1.CarState {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case string:
		switch val {
		case "INIT":
			return racestatev1.CarState_CAR_STATE_INIT
		case "RUN":
			return racestatev1.CarState_CAR_STATE_RUN
		case "PIT":
			return racestatev1.CarState_CAR_STATE_PIT
		case "OUT":
			return racestatev1.CarState_CAR_STATE_OUT
		case "SLOW":
			return racestatev1.CarState_CAR_STATE_SLOW
		case "FIN":
			return racestatev1.CarState_CAR_STATE_FIN

		default:
			return racestatev1.CarState_CAR_STATE_UNSPECIFIED
		}
	default:
		return racestatev1.CarState_CAR_STATE_UNSPECIFIED
	}
}

//nolint:whitespace // can't make both editor and linter happy
func convertMessageType(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) racestatev1.MessageType {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case string:
		switch val {
		case "Timing":
			return racestatev1.MessageType_MESSAGE_TYPE_TIMING
		case "Pits":
			return racestatev1.MessageType_MESSAGE_TYPE_PITS
		default:
			return racestatev1.MessageType_MESSAGE_TYPE_UNSPECIFIED
		}
	default:
		return racestatev1.MessageType_MESSAGE_TYPE_UNSPECIFIED
	}
}

//nolint:whitespace // can't make both editor and linter happy
func convertMessageSubType(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) racestatev1.MessageSubType {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case string:
		switch val {
		case "Driver":
			return racestatev1.MessageSubType_MESSAGE_SUB_TYPE_DRIVER
		case "RaceControl":
			return racestatev1.MessageSubType_MESSAGE_SUB_TYPE_RACE_CONTROL
		default:
			return racestatev1.MessageSubType_MESSAGE_SUB_TYPE_UNSPECIFIED
		}
	default:
		return racestatev1.MessageSubType_MESSAGE_SUB_TYPE_UNSPECIFIED
	}
}

//nolint:whitespace // can't make both editor and linter happy
func (p *StateConverter) getIntFrom(
	msg []interface{},
	key string,
	extractor func(arg []interface{}, key string) interface{},
) int {
	rawVal := extractor(msg, key)
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

//nolint:whitespace // can't make both editor and linter happy
func (p *StateConverter) getFloatFrom(
	msg []interface{},
	key string,
	extractor func(arg []interface{}, key string) interface{},
) float64 {
	rawVal := extractor(msg, key)
	switch val := rawVal.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	return -1
}

//nolint:whitespace // can't make both editor and linter happy
func (p *StateConverter) getStringFrom(
	msg []interface{},
	key string,
	extractor func(arg []interface{}, key string) interface{},
) string {
	rawVal := extractor(msg, key)
	switch val := rawVal.(type) {
	case string:
		return val
	default:
		fmt.Printf("Error extracting string val %s: %v %T\n", key, rawVal, rawVal)
	}
	return ""
}
