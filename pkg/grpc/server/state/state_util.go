package state

import (
	"context"
	"math"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	eventv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/event/v1"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/server/util"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

type (
	statesRequest interface {
		GetEvent() *commonv1.EventSelector
		GetStart() *commonv1.StartSelector
		GetNum() int32
	}
	statesContainer struct {
		e            *eventv1.Event
		ctx          context.Context
		conn         repository.Querier
		req          statesRequest
		remain       int
		lastRsInfoId int
	}
)

//nolint:whitespace // editor/linter issue
func createStatesContainer(
	ctx context.Context,
	conn repository.Querier,
	req statesRequest,
) (ret *statesContainer, err error) {
	ret = &statesContainer{
		ctx:    ctx,
		conn:   conn,
		req:    req,
		remain: math.MaxInt32,
	}
	ret.e, err = util.ResolveEvent(ctx, conn, req.GetEvent())
	if err != nil {
		return nil, err
	}
	if req.GetStart() == nil {
		return nil, util.ErrMissingStartSelector
	}
	if req.GetNum() > 0 {
		ret.remain = int(req.GetNum())
	}

	return ret, nil
}

func (s *statesContainer) toFetchEntries() int {
	if s.req.GetNum() > 0 {
		return min(min(1000, int(s.req.GetNum())), s.remain)
	}
	return 1000
}

func (s *statesContainer) getDefaultSessionNum() uint32 {
	if raceSession := utils.CollectRaceSessions(s.e); len(raceSession) == 0 {
		return 0
	} else {
		return raceSession[0]
	}
}
