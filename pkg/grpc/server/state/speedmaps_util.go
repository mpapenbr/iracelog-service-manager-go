//nolint:dupl // false positive
package state

import (
	"context"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	racestatev1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/racestate/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	mainUtil "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/util"
)

type (
	speedmapContainer struct {
		statesContainer
	}
)

//nolint:whitespace // editor/linter issue
func createSpeedmapContainer(
	ctx context.Context,
	repos api.Repositories,
	req StatesRequest,
) (ret *speedmapContainer, err error) {
	var sc *statesContainer
	sc, err = createStatesContainer(ctx, repos, req)
	if err != nil {
		return nil, err
	}
	ret = &speedmapContainer{statesContainer: *sc}
	return ret, nil
}

//nolint:whitespace,dupl // editor/linter issue
func (s *speedmapContainer) InitialRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishSpeedmapRequest],
	err error,
) {
	switch s.req.GetStart().Arg.(type) {
	case *commonv1.StartSelector_RecordStamp:
		ret, err = s.repos.Speedmap().LoadRange(
			s.ctx,
			int(s.e.Id),
			s.req.GetStart().GetRecordStamp().AsTime(),
			s.toFetchEntries())
	case *commonv1.StartSelector_SessionTime:
		ret, err = s.repos.Speedmap().LoadRangeBySessionTime(
			s.ctx,
			int(s.e.Id),
			s.getDefaultSessionNum(),
			float64(s.req.GetStart().GetSessionTime()),
			s.toFetchEntries())
	case *commonv1.StartSelector_SessionTimeSelector:
		ret, err = s.repos.Speedmap().LoadRangeBySessionTime(
			s.ctx,
			int(s.e.Id),
			uint32(s.req.GetStart().GetSessionTimeSelector().GetNum()),
			s.req.GetStart().GetSessionTimeSelector().GetDuration().AsDuration().Seconds(),
			s.toFetchEntries())
	case *commonv1.StartSelector_Id:
		ret, err = s.repos.Speedmap().LoadRangeByID(
			s.ctx,
			int(s.e.Id),
			int(s.req.GetStart().GetId()),
			s.toFetchEntries())
	default:
		ret = nil
		err = util.ErrInvalidStartSelector

	}
	if err == nil {
		s.lastRsInfoID = ret.LastRsInfoID
		s.remain -= len(ret.Data)
	}
	return ret, err
}

//nolint:whitespace,dupl // editor/linter issue
func (s *speedmapContainer) NextRequest() (
	ret *mainUtil.RangeContainer[racestatev1.PublishSpeedmapRequest],
	err error,
) {
	switch s.req.GetStart().Arg.(type) {
	case *commonv1.StartSelector_SessionTimeSelector:
		ret, err = s.repos.Speedmap().LoadRangeByIDWithinSession(
			s.ctx,
			int(s.e.Id),
			uint32(s.req.GetStart().GetSessionTimeSelector().GetNum()),
			s.lastRsInfoID+1,
			s.toFetchEntries())
	default:
		ret, err = s.repos.Speedmap().LoadRangeByID(
			s.ctx,
			int(s.e.Id),
			s.lastRsInfoID+1,
			s.toFetchEntries())
	}
	if err == nil {
		s.lastRsInfoID = ret.LastRsInfoID
		s.remain -= len(ret.Data)
	}
	return ret, err
}
