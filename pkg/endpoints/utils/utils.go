package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
)

type ClientConfigFunc func(cfg *client.Config)

func NewClient(opts ...ClientConfigFunc) (*client.Client, error) {
	cfg := client.Config{
		Realm: config.Realm,

		// Logger:       logger,
		HelloDetails: wamp.Dict{"authid": "backend"},
		AuthHandlers: map[string]client.AuthFunc{
			"ticket": func(*wamp.Challenge) (string, wamp.Dict) {
				return config.Password, wamp.Dict{}
			},
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	log.Info("Connecting to", log.String("url", config.URL))
	return client.ConnectNet(context.Background(), config.URL, cfg)
}

func WithClientLogging(logger *log.Logger) ClientConfigFunc {
	return func(cfg *client.Config) {
		cfg.Logger = myLogWrapper{logger: logger}
		cfg.Debug = logger.Level() == log.DebugLevel
	}
}

type myLogWrapper struct {
	logger *log.Logger
}

// Printf implements stdlog.StdLog.
func (l myLogWrapper) Printf(format string, v ...interface{}) {
	l.logger.Debug("nexus-client", log.String("msg", fmt.Sprintf(format, v...)))
}

// Println implements stdlog.StdLog.
func (l myLogWrapper) Println(v ...interface{}) {
	s := []string{}
	for _, val := range v {
		s = append(s, fmt.Sprintf("%v", val))
	}
	l.logger.Debug("nexus client", log.String("msg", strings.Join(s, " ")))
}

func (l myLogWrapper) Print(v ...interface{}) {
	l.Println(v...)
}

func ExtractEventKey(inv *wamp.Invocation) (string, error) {
	if len(inv.Arguments) != 1 {
		return "", fmt.Errorf("need exact 1 argument in request")
	}
	if _, ok := inv.Arguments[0].(string); ok {
		if ret, ok := wamp.AsString(inv.Arguments[0]); ok {
			return ret, nil
		} else {
			return "", fmt.Errorf("Cannot extract eventKey from message")
		}
	}
	return "", fmt.Errorf("invalid request in message")
}

func ExtractId(inv *wamp.Invocation) (int, error) {
	if len(inv.Arguments) != 1 {
		return -1, fmt.Errorf("need exact 1 argument in request")
	}
	return extractInt(inv.Arguments[0])
}

// extracts the argument on position pos (0-based) as an int
func ExtractIntArg(inv *wamp.Invocation, pos int) (int, error) {
	if len(inv.Arguments) < pos {
		return -1, fmt.Errorf("pos: %d not in range 0..%d", pos, len(inv.Arguments))
	}
	return extractInt(inv.Arguments[pos])
}

type RangeTuple struct {
	EventID int
	TsBegin float64
	Num     int
}

func ExtractRangeTuple(inv *wamp.Invocation) (*RangeTuple, error) {
	ret := RangeTuple{}

	if len(inv.Arguments) != 3 {
		return nil, fmt.Errorf("need exact 3 arguments in request")
	}
	var err error
	ret.EventID, err = extractInt(inv.Arguments[0])
	if err != nil {
		return nil, fmt.Errorf("parse eventId")
	}
	ret.TsBegin, err = extractFloat(inv.Arguments[1])
	if err != nil {
		return nil, fmt.Errorf("parse tsBegin")
	}
	ret.Num, err = extractInt(inv.Arguments[2])
	if err != nil {
		return nil, fmt.Errorf("parse num")
	}

	return &ret, nil
}

type ParamAvgLap struct {
	EventID      int
	IntervalSecs int
}

func ExtractParamAvgLap(inv *wamp.Invocation) (*ParamAvgLap, error) {
	ret := ParamAvgLap{}

	if len(inv.Arguments) != 2 {
		return nil, fmt.Errorf("need exact 2 arguments in request")
	}
	var err error
	ret.EventID, err = extractInt(inv.Arguments[0])
	if err != nil {
		return nil, fmt.Errorf("parse eventId")
	}
	ret.IntervalSecs, err = extractInt(inv.Arguments[1])
	if err != nil {
		return nil, fmt.Errorf("parse intervalSecs")
	}

	return &ret, nil
}

func extractInt(val any) (int, error) {
	switch t := val.(type) {
	case uint64:
		return int(t), nil
	case uint:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	}
	return -1, fmt.Errorf("not an integer or compatible value")
}

func extractFloat(val any) (float64, error) {
	switch t := val.(type) {
	case float32:
		return float64(t), nil
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case uint64:
		return float64(t), nil
	}
	return -1, fmt.Errorf("not a float or compatible value")
}
