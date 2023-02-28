package utils

import (
	"context"
	"fmt"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"go.uber.org/zap"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
)

func NewClient() (*client.Client, error) {
	logger := zap.NewStdLog(log.Logger)
	cfg := client.Config{
		Realm:        config.Realm,
		Logger:       logger,
		HelloDetails: wamp.Dict{"authid": "backend"},
		AuthHandlers: map[string]client.AuthFunc{
			"ticket": func(*wamp.Challenge) (string, wamp.Dict) {
				return config.Password, wamp.Dict{}
			},
		},
	}
	log.Logger.Info("Connecting to", zap.String("url", config.URL))
	return client.ConnectNet(context.Background(), config.URL, cfg)
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
	switch t := inv.Arguments[0].(type) {
	case uint64:
		return int(t), nil
	case uint:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	}

	return -1, fmt.Errorf("invalid request in message")
}
