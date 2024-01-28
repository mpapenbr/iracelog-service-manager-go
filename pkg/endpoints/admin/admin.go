package admin

import (
	"context"
	"fmt"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/service"
)

type (
	AdminManager struct {
		wampClient *client.Client
		aService   *service.AdminService
	}

	Option func(*AdminManager)
)

func WithPersistence(db *pgxpool.Pool) Option {
	return func(pm *AdminManager) {
		pm.aService = service.InitAdminService(db)
	}
}

func WithWampClient(wampClient *client.Client) Option {
	return func(pm *AdminManager) {
		pm.wampClient = wampClient
	}
}

func NewAdminManager(opts ...Option) (*AdminManager, error) {
	ret := &AdminManager{}
	for _, opt := range opts {
		opt(ret)
	}
	if ret.aService == nil {
		return nil, fmt.Errorf("no persistence layer provided")
	}
	if err := ret.handleDeleteEvent(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (admin *AdminManager) Shutdown() {
	log.Info("Unregister admin manager")
	for _, endpoint := range []string{
		"racelog.admin.event.delete",
	} {
		log.Info("Unregistering ", log.String("endpoint", endpoint))
		err := admin.wampClient.Unregister(endpoint)
		if err != nil {
			log.Error("Failed to unregister procedure:", log.ErrorField(err))
		}
	}
}

func (admin *AdminManager) handleDeleteEvent() error {
	return admin.wampClient.Register("racelog.admin.event.delete",
		func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
			id, err := utils.ExtractId(i)
			if err != nil {
				return client.InvokeResult{
					Args: wamp.List{"invalid delete event request"},
				}
			}
			log.Debug("delete event", log.Int("id", id))
			err = admin.aService.DeleteEvent(id)
			if err != nil {
				return client.InvokeResult{
					Args: wamp.List{fmt.Sprintf("error deleting event %d: %v", id, err)},
				}
			}
			return client.InvokeResult{Args: wamp.List{
				fmt.Sprintf("event %d removed", id),
			}}
		}, wamp.Dict{})
}
