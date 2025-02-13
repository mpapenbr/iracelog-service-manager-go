//nolint:dupl // false positive
package state

import (
	"context"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	carrepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/car/proto"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	mainUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

type (
	driverDataContainer struct {
		statesContainer
	}
)

//nolint:whitespace // editor/linter issue
func createDriverDataContainer(
	ctx context.Context,
	conn repository.Querier,
	req statesRequest,
) (ret *driverDataContainer, err error) {
	var sc *statesContainer
	sc, err = createStatesContainer(ctx, conn, req)
	if err != nil {
		return nil, err
	}
	ret = &driverDataContainer{statesContainer: *sc}
	return ret, nil
}

//nolint:whitespace,dupl // editor/linter issue
func (s *driverDataContainer) InitialRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishDriverDataRequest],
	err error,
) {
	switch s.req.GetStart().Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		ret, err = carrepos.LoadRange(
			s.ctx,
			s.conn,
			int(s.e.Id),
			s.req.GetStart().GetRecordStamp().AsTime(),
			s.toFetchEntries())
	case *commonv1.StartSelector_SessionTime:
		ret, err = carrepos.LoadRangeBySessionTime(
			s.ctx,
			s.conn,
			int(s.e.Id),
			s.getDefaultSessionNum(),
			float64(s.req.GetStart().GetSessionTime()),
			s.toFetchEntries())
	case *commonv1.StartSelector_SessionTimeSelector:
		ret, err = carrepos.LoadRangeBySessionTime(
			s.ctx,
			s.conn,
			int(s.e.Id),
			uint32(s.req.GetStart().GetSessionTimeSelector().GetNum()),
			s.req.GetStart().GetSessionTimeSelector().GetDuration().AsDuration().Seconds(),
			s.toFetchEntries())
	case *commonv1.StartSelector_Id:
		ret, err = carrepos.LoadRangeById(
			s.ctx,
			s.conn,
			int(s.e.Id),
			int(s.req.GetStart().GetId()),
			s.toFetchEntries())

	default:
		ret = nil
		err = util.ErrInvalidStartSelector

	}
	if err == nil {
		s.lastRsInfoId = ret.LastRsInfoId
		s.remain -= len(ret.Data)
	}
	return ret, err
}

//nolint:whitespace,dupl // editor/linter issue
func (s *driverDataContainer) NextRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishDriverDataRequest],
	err error,
) {
	switch s.req.GetStart().Arg.(type) {
	case *commonv1.StartSelector_SessionTimeSelector:
		ret, err = carrepos.LoadRangeByIdWithinSession(
			s.ctx,
			s.conn,
			int(s.e.Id),
			uint32(s.req.GetStart().GetSessionTimeSelector().GetNum()),
			s.lastRsInfoId+1,
			s.toFetchEntries())
	default:
		ret, err = carrepos.LoadRangeById(
			s.ctx,
			s.conn,
			int(s.e.Id),
			s.lastRsInfoId+1,
			s.toFetchEntries())
	}
	if err == nil {
		s.lastRsInfoId = ret.LastRsInfoId
		s.remain -= len(ret.Data)
	}
	return ret, err
}
