package proxy

import (
	"bytes"
	"encoding/binary"
	"fmt"

	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"google.golang.org/protobuf/proto"
)

func (e *EventData) ToBinary() ([]byte, error) {
	var result bytes.Buffer
	var data []byte
	var err error
	data, err = proto.Marshal(e.Event)
	if err != nil {
		return nil, fmt.Errorf("error marshaling event data: %w", err)
	}
	if err = binary.Write(&result, binary.LittleEndian, int32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to write event length: %w", err)
	}
	result.Write(data)

	data, err = proto.Marshal(e.Track)
	if err != nil {
		return nil, fmt.Errorf("error marshaling track data: %w", err)
	}
	if err := binary.Write(&result, binary.LittleEndian, int32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to write track length: %w", err)
	}
	result.Write(data)

	return result.Bytes(), nil
}

func (e *EventData) FromBinary(data []byte) error {
	var err error
	var event eventv1.Event

	buffer := bytes.NewReader(data)

	var msgLen int32
	// event
	if err = binary.Read(buffer, binary.LittleEndian, &msgLen); err != nil {
		return fmt.Errorf("failed to read A length: %w", err)
	}
	msgData := make([]byte, msgLen)
	if _, err = buffer.Read(msgData); err != nil {
		return fmt.Errorf("failed to read event data: %w", err)
	}
	err = proto.Unmarshal(msgData, &event)
	if err != nil {
		return fmt.Errorf("error unmarshaling event data: %w", err)
	}
	// track
	if err = binary.Read(buffer, binary.LittleEndian, &msgLen); err != nil {
		return fmt.Errorf("failed to read A length: %w", err)
	}
	msgData = make([]byte, msgLen)
	if _, err = buffer.Read(msgData); err != nil {
		return fmt.Errorf("failed to read track data: %w", err)
	}
	var track trackv1.Track
	err = proto.Unmarshal(msgData, &track)
	if err != nil {
		return fmt.Errorf("error unmarshaling track: %w", err)
	}

	e.Event = &event
	e.Track = &track
	return nil
}

func (e *EventData) Equals(other *EventData) bool {
	if e == nil || other == nil {
		return false
	}
	return proto.Equal(e.Event, other.Event) && proto.Equal(e.Track, other.Track)
}
