//nolint:dupl // false positive
package state

import (
	"context"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	racestaterepos "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/racestate"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	mainUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

type (
	racestatesContainer struct {
		statesContainer
	}
)

//nolint:whitespace // editor/linter issue
func createRacestatesContainer(
	ctx context.Context,
	conn repository.Querier,
	req statesRequest,
) (ret *racestatesContainer, err error) {
	var sc *statesContainer
	sc, err = createStatesContainer(ctx, conn, req)
	if err != nil {
		return nil, err
	}
	ret = &racestatesContainer{statesContainer: *sc}
	return ret, nil
}

//nolint:whitespace,dupl // editor/linter issue
func (s *racestatesContainer) InitialRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishStateRequest],
	err error,
) {
	switch s.req.GetStart().Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		ret, err = racestaterepos.LoadRange(
			s.ctx,
			s.conn,
			int(s.e.Id),
			s.req.GetStart().GetRecordStamp().AsTime(),
			s.toFetchEntries())
	case *commonv1.StartSelector_SessionTime:
		ret, err = racestaterepos.LoadRangeBySessionTime(
			s.ctx,
			s.conn,
			int(s.e.Id),
			float64(s.req.GetStart().GetSessionTime()),
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
func (s *racestatesContainer) NextRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishStateRequest],
	err error,
) {
	ret, err = racestaterepos.LoadRangeById(
		s.ctx,
		s.conn,
		int(s.e.Id),
		s.lastRsInfoId+1,
		s.toFetchEntries())
	if err == nil {
		s.lastRsInfoId = ret.LastRsInfoId
		s.remain -= len(ret.Data)
	}
	return ret, err
}

func (s *racestatesContainer) toFetchEntries() int {
	if s.req.GetNum() > 0 {
		return min(min(2000, int(s.req.GetNum())), s.remain)
	}
	return 2000
}
