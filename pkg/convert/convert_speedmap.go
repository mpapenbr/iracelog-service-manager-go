package convert

import (
	speedmapv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/speedmap/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

func NewSpeedmapMessageConverter() *SpeedmapMessageConverter {
	return &SpeedmapMessageConverter{}
}

type SpeedmapMessageConverter struct{}

func (c *SpeedmapMessageConverter) ConvertSpeedmapPayload(
	in *model.SpeedmapData,
) *speedmapv1.Speedmap {
	return &speedmapv1.Speedmap{
		ChunkSize:      uint32(in.Payload.ChunkSize),
		TimeOfDay:      uint32(in.Payload.TimeOfDay),
		TrackTemp:      float32(in.Payload.TrackTemp),
		SessionTime:    float32(in.Payload.SessionTime),
		LeaderTrackPos: float32(in.Payload.CurrentPos),
		Data:           c.convertData(in.Payload.Data),
	}
}

func (c *SpeedmapMessageConverter) convertData(data map[string]*model.ClassSpeedmapData) map[string]*speedmapv1.ClassData {
	ret := make(map[string]*speedmapv1.ClassData)
	for k, v := range data {
		ret[k] = &speedmapv1.ClassData{
			Laptime:     float32(v.Laptime),
			ChunkSpeeds: c.convertChunkSpeeds(v.ChunkSpeeds),
		}
	}
	return ret
}

func (c *SpeedmapMessageConverter) convertChunkSpeeds(in []float64) []float32 {
	ret := make([]float32, len(in))
	for i, v := range in {
		ret[i] = float32(v)
	}
	return ret
}
