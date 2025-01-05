package nats

import (
	"bytes"
	"encoding/binary"
	"fmt"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
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
