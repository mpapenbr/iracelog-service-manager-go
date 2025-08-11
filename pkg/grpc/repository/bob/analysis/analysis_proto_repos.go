//nolint:whitespace // can't make both editor and linter happy
package analysis

import (
	"context"
	"time"

	analysisv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/analysis/v1"
	"github.com/aarondl/opt/omit"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"google.golang.org/protobuf/proto"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.AnalysisRepository = (*repo)(nil)

func NewAnalysisRepository(conn bob.Executor) api.AnalysisRepository {
	return &repo{
		conn: conn,
	}
}

func (r *repo) Upsert(
	ctx context.Context,
	eventID int,
	analysis *analysisv1.Analysis,
) error {
	binaryMessage, err := proto.Marshal(analysis)
	if err != nil {
		return err
	}
	setter := &models.AnalysisProtoSetter{
		EventID:     omit.From(int32(eventID)),
		Protodata:   omit.From(binaryMessage),
		RecordStamp: omit.From(time.Now()),
	}
	_, err = models.AnalysisProtos.Insert(setter,
		im.OnConflict(models.ColumnNames.AnalysisProtos.EventID).DoUpdate(
			im.SetCol(models.ColumnNames.AnalysisProtos.Protodata).To(
				psql.Arg(binaryMessage),
			),
			im.SetCol(models.ColumnNames.AnalysisProtos.RecordStamp).To(
				psql.Arg(time.Now()),
			),
		)).One(ctx, r.getExecutor(ctx))
	return err
}

func (r *repo) LoadByEventID(
	ctx context.Context,
	eventID int,
) (*analysisv1.Analysis, error) {
	query := models.AnalysisProtos.Query(
		models.SelectWhere.AnalysisProtos.EventID.EQ(int32(eventID)),
	)
	res, err := query.One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}

	analysis := &analysisv1.Analysis{}
	if err := proto.Unmarshal(res.Protodata, analysis); err != nil {
		return nil, err
	}

	return analysis, nil
}

//nolint:lll // readability
func (r *repo) LoadByEventKey(
	ctx context.Context,
	eventKey string,
) (*analysisv1.Analysis, error) {
	subQuery := psql.Select(
		sm.Columns(models.ColumnNames.Events.ID),
		sm.From(models.TableNames.Events),
		models.SelectWhere.Events.EventKey.EQ(eventKey),
	)
	res, err := models.AnalysisProtos.Query(
		sm.Where(models.AnalysisProtoColumns.EventID.EQ(
			subQuery,
		)),
	).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}

	analysis := &analysisv1.Analysis{}
	if err := proto.Unmarshal(res.Protodata, analysis); err != nil {
		return nil, err
	}

	return analysis, nil
}

// deletes an entry from the database, returns number of rows deleted.
func (r *repo) DeleteByEventID(ctx context.Context, eventID int) (int, error) {
	ret, err := models.AnalysisProtos.Delete(
		models.DeleteWhere.AnalysisProtos.EventID.EQ(int32(eventID)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
