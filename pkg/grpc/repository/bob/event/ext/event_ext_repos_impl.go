//nolint:whitespace // can't make both editor and linter happy
package ext

import (
	"context"

	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"
	"github.com/aarondl/opt/omit"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/mytypes"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.EventExtRepository = (*repo)(nil)

func NewEventExtRepository(conn bob.Executor) api.EventExtRepository {
	return &repo{
		conn: conn,
	}
}

func (r *repo) Upsert(
	ctx context.Context,
	eventID int,
	extra *racestatev1.ExtraInfo,
) error {
	var err error

	setter := &models.EventExtSetter{
		EventID: omit.From(int32(eventID)),
		ExtraInfo: omit.From(mytypes.CustomExtraInfo{
			PitInfo: extra.PitInfo,
		}),
	}
	_, err = models.EventExts.Insert(setter,
		im.OnConflict(models.ColumnNames.EventExts.EventID).DoUpdate(
			im.SetCol(models.ColumnNames.EventExts.ExtraInfo).To(
				psql.Arg(extra),
			),
		)).One(ctx, r.getExecutor(ctx))
	return err
}

func (r *repo) LoadByEventID(
	ctx context.Context,
	eventID int,
) (*racestatev1.ExtraInfo, error) {
	res, err := models.EventExts.Query(
		models.SelectWhere.EventExts.EventID.EQ(int32(eventID))).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}

	return &racestatev1.ExtraInfo{
		PitInfo: res.ExtraInfo.PitInfo,
	}, nil
}

// deletes an entry from the database, returns number of rows deleted.
//
//nolint:lll // readability
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (int, error) {
	ret, err := models.EventExts.Delete(
		models.DeleteWhere.EventExts.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
