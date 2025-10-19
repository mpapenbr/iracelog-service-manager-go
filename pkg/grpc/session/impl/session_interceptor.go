package impl

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
)

type (
	sessionInterceptor struct {
		sessionStore session.SessionStore
		l            *log.Logger
	}
)

// returns an interceptor that adds a cookie to the response if the context
// contains a session
func NewSessionInterceptor(store session.SessionStore) connect.Interceptor {
	ret := &sessionInterceptor{
		l:            log.Default().Named("grpc.session"),
		sessionStore: store,
	}

	return ret
}

//nolint:whitespace // can't make both editor and linter happy
func (i *sessionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		dummyReq := http.Request{Header: req.Header()}
		if myCookie, err := dummyReq.Cookie(session.SessionIDCookie); err == nil {
			i.l.Debug("found session cookie", log.String("cookie", myCookie.String()))
			sessionID := myCookie.Value
			if s, sErr := i.sessionStore.Get(sessionID); sErr == nil {
				ctx = session.AddSessionToContext(ctx, s)
			} else {
				i.l.Debug("error getting session from store", log.ErrorField(sErr))
			}
		}

		res, err := next(ctx, req)
		if err != nil {
			return nil, err
		}

		// if there is already a cookie in the response, do not overwrite it
		// this is most likely set by login/logout
		if res.Header().Get("Set-Cookie") != "" {
			i.l.Debug("keeping already set cookie")
			return res, nil
		}

		sessionData := session.SessionFromContext(ctx)
		if sessionData != nil {
			i.l.Debug("adding session cookie", log.String("session_id", sessionData.ID()))
			res.Header().Add("Set-Cookie",
				session.CreateCookieForSession(sessionData, i.sessionStore.Timeout()).String())
		}

		return res, nil
	})
}

//
//nolint:whitespace // editor/linter issue
func (i *sessionInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

//
//nolint:lll,whitespace // better readability
func (i *sessionInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		return next(ctx, conn)
	})
}
