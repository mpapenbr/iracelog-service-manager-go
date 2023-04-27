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

type AdminManager struct {
	wampClient *client.Client
	aService   *service.AdminService
}

func InitAdminEndpoints(pool *pgxpool.Pool) (*AdminManager, error) {
	wampClient, err := utils.NewClient()
	if err != nil {
		log.Fatal("Could not connect wamp client", log.ErrorField(err))
	}
	ret := AdminManager{
		wampClient: wampClient,
		aService:   service.InitAdminService(pool),
	}

	if err := ret.handleDeleteEvent(); err != nil {
		return nil, err
	}

	return &ret, nil
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
