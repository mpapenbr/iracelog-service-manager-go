//nolint:whitespace // can't make both editor and linter happy
package track

import (
	"context"
	"database/sql"
	"errors"

	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"github.com/aarondl/opt/omit"
	"github.com/shopspring/decimal"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/sm"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/mytypes"
	api "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.TrackRepository = (*repo)(nil)

func NewTrackRepository(conn bob.Executor) api.TrackRepository {
	return &repo{
		conn: conn,
	}
}

//nolint:whitespace // editor/linter issue
func (r *repo) Create(ctx context.Context, track *trackv1.Track) error {
	workPitInfo := track.PitInfo
	if track.PitInfo == nil {
		workPitInfo = &trackv1.PitInfo{}
	}

	setter := &models.TrackSetter{
		ID:            omit.From(int32(track.Id)),
		Name:          omit.From(track.Name),
		ShortName:     omit.From(track.ShortName),
		Config:        omit.From(track.Config),
		TrackLength:   omit.From(decimal.NewFromFloat32(track.Length)),
		Sectors:       omit.From(mytypes.SectorSlice(track.Sectors)),
		PitSpeed:      omit.From(decimal.NewFromFloat32(track.PitSpeed)),
		PitEntry:      omit.From(decimal.NewFromFloat32(workPitInfo.Entry)),
		PitExit:       omit.From(decimal.NewFromFloat32(workPitInfo.Exit)),
		PitLaneLength: omit.From(decimal.NewFromFloat32(workPitInfo.LaneLength)),
	}
	_, err := models.Tracks.Insert(setter).One(ctx, r.getExecutor(ctx))
	return err
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByID(ctx context.Context, id int) (
	*trackv1.Track, error,
) {
	ret, err := models.Tracks.Query(
		models.SelectWhere.Tracks.ID.EQ(int32(id))).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toPBMessage(ret)
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadAll(ctx context.Context) (
	[]*trackv1.Track, error,
) {
	query := models.Tracks.Query(
		sm.OrderBy(models.Tracks.Columns.ID).Asc(),
	)
	data, err := query.All(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	ret := make([]*trackv1.Track, 0)

	for i := range data {
		item, err := r.toPBMessage(data[i])
		if err != nil {
			return nil, err
		}
		ret = append(ret, item)
	}
	return ret, nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r *repo) EnsureTrack(
	ctx context.Context,
	track *trackv1.Track,
) error {
	_, err := r.LoadByID(ctx, int(track.Id))
	if errors.Is(err, sql.ErrNoRows) {
		return r.Create(ctx, track)
	}
	return nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r repo) UpdatePitInfo(
	ctx context.Context,

	id int,
	pitInfo *trackv1.PitInfo) (
	int, error,
) {
	if pitInfo == nil {
		return 0, nil
	}
	setter := &models.TrackSetter{
		PitEntry:      omit.From(decimal.NewFromFloat32(pitInfo.Entry)),
		PitExit:       omit.From(decimal.NewFromFloat32(pitInfo.Exit)),
		PitLaneLength: omit.From(decimal.NewFromFloat32(pitInfo.LaneLength)),
	}

	ret, err := models.Tracks.Update(
		setter.UpdateMod(),
		models.UpdateWhere.Tracks.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

// deletes an entry from the database, returns number of rows deleted.
func (r *repo) DeleteByID(ctx context.Context, id int) (int, error) {
	ret, err := models.Tracks.Delete(
		models.DeleteWhere.Tracks.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

func (r *repo) toPBMessage(dbTrack *models.Track) (*trackv1.Track, error) {
	var item trackv1.Track

	item.Id = uint32(dbTrack.ID)
	item.Name = dbTrack.Name
	item.ShortName = dbTrack.ShortName
	item.Config = dbTrack.Config
	item.Length = float32(dbTrack.TrackLength.Abs().InexactFloat64())
	item.PitSpeed = float32(dbTrack.PitSpeed.Abs().InexactFloat64())
	item.PitInfo = &trackv1.PitInfo{
		Entry:      float32(dbTrack.PitEntry.InexactFloat64()),
		Exit:       float32(dbTrack.PitExit.InexactFloat64()),
		LaneLength: float32(dbTrack.PitLaneLength.InexactFloat64()),
	}
	if dbTrack.Sectors != nil {
		item.Sectors = dbTrack.Sectors
	}

	return &item, nil
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
