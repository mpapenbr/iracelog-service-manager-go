package impl

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect/v2"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
)

type (
	sessionInterceptor struct {
		sessionStore session.SessionStore
		l            *log.Logger
	}
)

// returns an interceptor that adds a cookie to the response if the context
// contains a session
func NewSessionInterceptor(store session.SessionStore) connect.ServerInterceptor {
	i := &sessionInterceptor{
		l:            log.Default().Named("grpc.session"),
		sessionStore: store,
	}
	return i.intercept
}

//nolint:whitespace // can't make both editor and linter happy
func (i *sessionInterceptor) intercept(next connect.ServerFunc) connect.ServerFunc {
	return func(
		ctx context.Context,
		spec connect.Spec,
		stream connect.ServerStream,
	) error {
		// like in v1: session handling is only done for unary calls
		if spec.StreamType != connect.StreamTypeUnary {
			return next(ctx, spec, stream)
		}
		dummyReq := http.Request{Header: utils.RequestHeaderFromServerContext(ctx)}
		if myCookie, err := dummyReq.Cookie(i.sessionStore.CookieName()); err == nil {
			i.l.Debug("found session cookie", log.String("cookie", myCookie.String()))
			sessionID := myCookie.Value
			if s, sErr := i.sessionStore.Get(sessionID); sErr == nil {
				ctx = session.AddSessionToContext(ctx, s)
			} else {
				i.l.Debug("error getting session from store", log.ErrorField(sErr))
			}
		}
		// response headers are flushed with the first Send (called by the generated
		// unary handler after our service method returned), so the cookie has to
		// be added right before that.
		return next(ctx, spec, &sessionCookieStream{
			ServerStream: stream,
			onSend:       func() { i.addSessionCookie(ctx) },
		})
	}
}

func (i *sessionInterceptor) addSessionCookie(ctx context.Context) {
	info, ok := connect.CallInfoForServerContext(ctx)
	if !ok {
		return
	}
	// if there is already a cookie in the response, do not overwrite it
	// this is most likely set by login/logout
	if info.ResponseHeader().Get("Set-Cookie") != "" {
		i.l.Debug("keeping already set cookie")
		return
	}

	sessionData := session.SessionFromContext(ctx)
	if sessionData != nil {
		i.l.Debug("adding session cookie", log.String("session_id", sessionData.ID()))
		info.ResponseHeader().Add("Set-Cookie",
			session.CreateCookieForSession(
				i.sessionStore.CookieName(),
				sessionData,
				i.sessionStore.Timeout(),
			).String())
	}
}

// sessionCookieStream calls onSend once before the response headers are flushed
type sessionCookieStream struct {
	connect.ServerStream
	onSend func()
	once   sync.Once
}

func (s *sessionCookieStream) SendHeaders() error {
	s.once.Do(s.onSend)
	return s.ServerStream.SendHeaders()
}

func (s *sessionCookieStream) Send(msg any) error {
	s.once.Do(s.onSend)
	return s.ServerStream.Send(msg)
}
