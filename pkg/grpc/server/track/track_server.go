package track

import (
	"context"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/track/v1/trackv1connect"
	trackv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/track/v1"
	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
)

func NewServer(opts ...Option) *trackServer {
	ret := &trackServer{
		log: log.Default().Named("grpc.track"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}
	return ret
}

type Option func(*trackServer)

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *trackServer) {
		srv.pe = pe
	}
}

func WithTrackRepository(repo api.TrackRepository) Option {
	return func(srv *trackServer) {
		srv.trackRepos = repo
	}
}

func WithTxManager(repo api.TransactionManager) Option {
	return func(srv *trackServer) {
		srv.txManager = repo
	}
}

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *trackServer) {
		srv.tracer = tracer
	}
}

type trackServer struct {
	x.UnimplementedTrackServiceHandler
	pe         permission.PermissionEvaluator
	log        *log.Logger
	trackRepos api.TrackRepository
	txManager  api.TransactionManager
	tracer     trace.Tracer
}

//nolint:whitespace // can't make both editor and linter happy
func (s *trackServer) GetTracks(
	ctx context.Context,
	req *connect.Request[trackv1.GetTracksRequest],
	stream *connect.ServerStream[trackv1.GetTracksResponse],
) error {
	data, err := s.trackRepos.LoadAll(ctx)
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
	data, err := s.trackRepos.LoadByID(context.Background(), int(req.Msg.Id))
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
	if err := s.txManager.RunInTx(ctx, func(ctx context.Context) error {
		return s.trackRepos.EnsureTrack(ctx, req.Msg.Track)
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

	if err := s.txManager.RunInTx(ctx, func(ctx context.Context) error {
		_, updErr := s.trackRepos.UpdatePitInfo(ctx, int(req.Msg.Id), req.Msg.PitInfo)
		return updErr
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&trackv1.UpdatePitInfoResponse{}), nil
}
