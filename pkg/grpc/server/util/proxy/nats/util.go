package nats

import (
	"bytes"
	"encoding/binary"
	"fmt"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	containerv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/container/v1"
	"google.golang.org/protobuf/proto"
)

type snapHist struct{}

func (s snapHist) ToBinary(items []*analysisv1.SnapshotData) ([]byte, error) {
	var result bytes.Buffer
	for _, snap := range items {
		data, err := proto.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("error marshaling snapshot data: %w", err)
		}
		dataLen := uint32(len(data))
		err = binary.Write(&result, binary.LittleEndian, dataLen)
		if err != nil {
			return nil, fmt.Errorf("error writing snapshot data length: %w", err)
		}
		_, err = result.Write(data)
		if err != nil {
			return nil, fmt.Errorf("error writing snapshot data: %w", err)
		}
	}
	return result.Bytes(), nil
}

func (s snapHist) FromBinary(data []byte) ([]*analysisv1.SnapshotData, error) {
	var result []*analysisv1.SnapshotData
	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		var dataLen uint32
		err := binary.Read(buf, binary.LittleEndian, &dataLen)
		if err != nil {
			return nil, fmt.Errorf("error reading snapshot data length: %w", err)
		}
		data := make([]byte, dataLen)
		_, err = buf.Read(data)
		if err != nil {
			return nil, fmt.Errorf("error reading snapshot data: %w", err)
		}
		snap := &analysisv1.SnapshotData{}
		err = proto.Unmarshal(data, snap)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling snapshot data: %w", err)
		}
		result = append(result, snap)
	}
	return result, nil
}

type eventLookupTransfer struct{}

//nolint:whitespace // editor/linter issue
func (s eventLookupTransfer) ToBinary(input map[string]*containerv1.EventContainer) (
	ret []byte, err error,
) {
	var result bytes.Buffer
	for k, v := range input {
		if err = binary.Write(&result, binary.LittleEndian, uint32(len(k))); err != nil {
			return nil, fmt.Errorf("error writing event key length: %w", err)
		}
		_, err = result.Write([]byte(k))
		if err != nil {
			return nil, fmt.Errorf("error writing key: %w", err)
		}

		data, err := proto.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("error marshaling EventData: %w", err)
		}

		if err = binary.Write(
			&result,
			binary.LittleEndian,
			uint32(len(data))); err != nil {
			return nil, fmt.Errorf("error writing EventData  length: %w", err)
		}

		_, err = result.Write(data)
		if err != nil {
			return nil, fmt.Errorf("error writing EventData: %w", err)
		}
	}
	return result.Bytes(), nil
}

//nolint:whitespace // editor/linter issue
func (s eventLookupTransfer) FromBinary(data []byte) (
	ret map[string]*containerv1.EventContainer,
	err error,
) {
	result := make(map[string]*containerv1.EventContainer)
	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		var keyLen uint32
		if err = binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
			return nil, fmt.Errorf("error reading map key len: %w", err)
		}

		key := make([]byte, keyLen)
		if _, err = buf.Read(key); err != nil {
			return nil, fmt.Errorf("error reading key: %w", err)
		}

		var dataLen uint32
		if err = binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
			return nil, fmt.Errorf("error reading map key len: %w", err)
		}

		edBuf := make([]byte, dataLen)
		if _, err = buf.Read(edBuf); err != nil {
			return nil, fmt.Errorf("error reading EventData: %w", err)
		}

		var eventData containerv1.EventContainer
		if err = proto.Unmarshal(edBuf, &eventData); err != nil {
			return nil, fmt.Errorf("error unmarshalling EventData: %w", err)
		}
		result[string(key)] = &eventData
	}
	return result, nil
}
