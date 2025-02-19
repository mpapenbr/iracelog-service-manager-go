package track

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/track/v1/trackv1connect"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/track"
)

func NewServer(opts ...Option) *trackServer {
	ret := &trackServer{
		log: log.Default().Named("grpc.track"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*trackServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *trackServer) {
		srv.pool = p
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *trackServer) {
		srv.pe = pe
	}
}

type trackServer struct {
	x.UnimplementedTrackServiceHandler

	pe   permission.PermissionEvaluator
	pool *pgxpool.Pool
	log  *log.Logger
}

//nolint:whitespace // can't make both editor and linter happy
func (s *trackServer) GetTracks(
	ctx context.Context,
	req *connect.Request[trackv1.GetTracksRequest],
	stream *connect.ServerStream[trackv1.GetTracksResponse],
) error {
	data, err := track.LoadAll(context.Background(), s.pool)
	if err != nil {
		return err
	}
	for i := range data {
		if err := stream.Send(
			&trackv1.GetTracksResponse{Track: data[i]}); err != nil {
			s.log.Error("Error sending track", log.ErrorField(err))
			return err
		}
	}
	return nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *trackServer) GetTrack(
	ctx context.Context,
	req *connect.Request[trackv1.GetTrackRequest],
) (*connect.Response[trackv1.GetTrackResponse], error) {
	data, err := track.LoadById(context.Background(), s.pool, int(req.Msg.Id))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&trackv1.GetTrackResponse{Track: data}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *trackServer) EnsureTrack(
	ctx context.Context,
	req *connect.Request[trackv1.EnsureTrackRequest],
) (*connect.Response[trackv1.EnsureTrackResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionCreateTrack) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return track.EnsureTrack(ctx, tx, req.Msg.Track)
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&trackv1.EnsureTrackResponse{}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *trackServer) UpdatePitInfo(
	ctx context.Context,
	req *connect.Request[trackv1.UpdatePitInfoRequest],
) (*connect.Response[trackv1.UpdatePitInfoResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionUpdateTrack) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}

	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, updErr := track.UpdatePitInfo(
			context.Background(),
			s.pool,
			int(req.Msg.Id),
			req.Msg.PitInfo)
		return updErr
	}); err != nil {
		return nil, err
	}

	return connect.NewResponse(&trackv1.UpdatePitInfoResponse{}), nil
}
