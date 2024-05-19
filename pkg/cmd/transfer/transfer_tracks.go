//nolint:all // temporary command, not designed to stay in the codebase
package transfer

import (
	"context"

	trackv1 "buf.build/gen/go/mpapenbr/testrepo/protocolbuffers/go/testrepo/track/v1"
	"github.com/mpapenbr/iracelog-service-manager-go/log"
	trackDest "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	trackSource "github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
	"github.com/spf13/cobra"
)

func NewTransferTracksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tracks",
		Short: "converts tracks to new database",
		Run: func(cmd *cobra.Command, args []string) {
			transferTracks()
		},
	}

	return cmd
}

func transferTracks() {
	parseLogLevel(logLevel, log.InfoLevel)
	setupDatabases()
	defer poolSource.Close()
	defer poolDest.Close()

	convertSectors := func(secs []model.Sector) []*trackv1.Sector {
		ret := make([]*trackv1.Sector, len(secs))
		for i, s := range secs {
			ret[i] = &trackv1.Sector{
				Num:      uint32(s.SectorNum),
				StartPct: float32(s.SectorStartPct),
			}
		}
		return ret
	}
	getPitLaneLength := func(pit *model.PitInfo) float32 {
		if pit.LaneLength > 0 {
			return float32(pit.LaneLength)
		} else if pit.Lane > 0 {
			return float32(pit.Lane)
		} else {
			return 0
		}
	}

	log.Info("Transferring tracks")

	sourceTracks, err := trackSource.LoadAll(context.Background(), poolSource)
	if err != nil {
		log.Error("error loading tracks from source", log.ErrorField(err))
		return
	}
	for _, t := range sourceTracks {
		log.Info("transferring track", log.Int("id", t.ID), log.String("name", t.Data.Name))
		destTrack := &trackv1.Track{
			Id:        &trackv1.TrackId{Id: uint32(t.Data.ID)},
			Name:      t.Data.Name,
			ShortName: t.Data.ShortName,
			Config:    t.Data.Config,
			Length:    float32(t.Data.Length),

			Sectors: convertSectors(t.Data.Sectors),
			PitInfo: &trackv1.PitInfo{
				Entry:      float32(t.Data.Pit.Entry),
				Exit:       float32(t.Data.Pit.Exit),
				LaneLength: getPitLaneLength(t.Data.Pit),
			},
		}
		if err := trackDest.Create(context.Background(), poolDest, destTrack); err != nil {
			log.Error("error transferring track", log.ErrorField(err))
		}

	}
}
