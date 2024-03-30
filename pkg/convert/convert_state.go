package convert

import (
	"fmt"
	"time"

	eventv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/event/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/racestate/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/processing/util"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewStateMessageConverter(e *model.DbEvent, eventKey string) *StateConverter {
	ret := &StateConverter{
		payloadExtractor: util.NewPayloadExtractor(&e.Data.Manifests),
		eventKey:         eventKey,
	}
	return ret
}

type StateConverter struct {
	payloadExtractor *util.PayloadExtractor
	eventKey         string
}

func (p *StateConverter) ConvertStatePayload(m *model.StateData) *racestatev1.PublishStateRequest {
	ret := &racestatev1.PublishStateRequest{
		Event:     &eventv1.EventSelector{Arg: &eventv1.EventSelector_Key{Key: p.eventKey}},
		Cars:      p.convertCars(m.Payload.Cars),
		Session:   p.convertSession(m.Payload.Session),
		Messages:  p.convertMessages(m.Payload.Messages),
		Timestamp: timestamppb.New(time.UnixMilli(int64(m.Timestamp))),
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
		TimeRemain:      float32(p.getFloatFrom(session, "timeRemain", extr)),
		LapsRemain:      int32(p.getIntFrom(session, "lapsRemain", extr)),
		FlagState:       p.getStringFrom(session, "flagState", extr),
		SessionStateRaw: int32(p.getIntFrom(session, "sessionStateRaw", extr)),
		SessionFlagsRaw: uint32(p.getIntFrom(session, "sessionFlagsRaw", extr)),
	}
}

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

func convertTireCompound(
	msg []interface{}, key string, extr func(arg []interface{}, key string) interface{},
) *racestatev1.TireCompound {
	rawVal := extr(msg, key)
	switch val := rawVal.(type) {
	case int:
		return &racestatev1.TireCompound{RawValue: uint32(val)}
	default:
		return nil
	}
}

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

func (p *StateConverter) getLaptime(
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
