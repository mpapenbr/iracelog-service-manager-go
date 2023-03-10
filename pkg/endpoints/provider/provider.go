package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/service"
)

type ProviderManager struct {
	pService        *service.ProviderService
	stateService    *service.StateService
	speedmapService *service.SpeedmapService
	carService      *service.CarService
	wampClient      *client.Client
}

// contains the data sent to the client when using provider endpoints
type ProviderResponseData struct {
	EventKey   string              `json:"eventKey"`
	Manifests  model.Manifests     `json:"manifests"`
	Info       model.EventDataInfo `json:"info"`
	RecordDate time.Time           `json:"recordDate"`
	DbId       int                 `json:"dbId"`
}

func InitProviderEndpoints(pool *pgxpool.Pool) (*ProviderManager, error) {
	wampClient, err := utils.NewClient()
	if err != nil {
		log.Fatal("Could not connect wamp client", log.ErrorField(err))
	}

	ret := &ProviderManager{
		pService:        service.InitProviderService(pool),
		stateService:    service.InitStateService(pool),
		speedmapService: service.InitSpeedmapService(pool),
		carService:      service.InitCarService(pool),
		wampClient:      wampClient,
	}
	if err := ret.handleRegisterProvider(); err != nil {
		return nil, err
	}
	if err := ret.handleRemoveProvider(); err != nil {
		return nil, err
	}
	if err := ret.handleListProvider(); err != nil {
		return nil, err
	}
	if err := ret.handleEventExtraData(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (pm *ProviderManager) Shutdown() {
	log.Info("Unregister provider manager")
	for _, endpoint := range []string{
		"racelog.dataprovider.register_provider",
		"racelog.dataprovider.remove_provider",
		"racelog.public.list_providers",
	} {
		log.Info("Unregistering ", log.String("endpoint", endpoint))
		err := pm.wampClient.Unregister(endpoint)
		if err != nil {
			log.Error("Failed to unregister procedure:", log.ErrorField(err))
		}
	}
}

func (pm *ProviderManager) handleListProvider() error {
	return pm.wampClient.Register("racelog.public.list_providers",
		func(ctx context.Context, inv *wamp.Invocation) client.InvokeResult {
			log.Info("Received list provider request")

			ret := make([]*ProviderResponseData, len(pm.pService.Lookup))
			idx := 0
			for _, v := range pm.pService.Lookup {
				ret[idx] = createProviderResponseData(v)
				idx += 1
			}

			return client.InvokeResult{Args: wamp.List{ret}}
		}, wamp.Dict{})
}

func (pm *ProviderManager) handleRegisterProvider() error {
	return pm.wampClient.Register("racelog.dataprovider.register_provider",
		func(ctx context.Context, inv *wamp.Invocation) client.InvokeResult {
			log.Info("Received register provider request")

			req, err := extractRegisterRequest(inv)
			if err != nil {
				return client.InvokeResult{Args: wamp.List{"invalid registration request"}}
			}
			log.Debug("reveiced data", log.Any("data", req))
			providerData, err := pm.pService.RegisterEvent(req)
			if err != nil {
				return client.InvokeResult{Args: wamp.List{"could not register event"}}
			}
			pm.addHandlers(providerData)
			pm.publishNewProvider(providerData)

			return client.InvokeResult{Args: wamp.List{providerData.Event}}
		}, wamp.Dict{})
}

// publish the new provider on the internal manager topic
func (pm *ProviderManager) publishNewProvider(pd *service.ProviderData) {
	log.Debug("publish new provider", log.String("eventKey", pd.Event.Key))
	msg := PublishNew{Type: Register, Payload: NewProviderPayload{
		EventKey:  pd.Event.Key,
		Info:      pd.Event.Data.Info,
		Manifests: pd.Event.Data.Manifests,
	}}
	err := pm.wampClient.Publish("racelog.manager.provider", nil, wamp.List{msg}, nil)
	if err != nil {
		log.Warn("Publish new provider", log.ErrorField(err))
	}
}

func (pm *ProviderManager) publishRemovedProvider(pd *service.ProviderData) {
	log.Debug("publish removed provider", log.String("eventKey", pd.Event.Key))
	msg := PublishRemoved{Type: Removed, Payload: pd.Event.Key}
	err := pm.wampClient.Publish("racelog.manager.provider", nil, wamp.List{msg}, nil)
	if err != nil {
		log.Warn("Publish removed provider", log.ErrorField(err))
	}
}

//nolint:funlen // by design
func (pm *ProviderManager) addHandlers(pd *service.ProviderData) {
	cli, err := utils.NewClient()
	if err != nil {
		log.Error("Could not establish data handlers for event",
			log.ErrorField(err))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	pd.ActiveClient = &service.Client{WampClient: cli, CancelFunc: cancel}

	topics := []struct {
		topic   string
		handler client.EventHandler
	}{
		{
			topic:   fmt.Sprintf("racelog.public.live.state.%s", pd.Event.Key),
			handler: pm.stateMessageHandler(pd),
		}, {
			topic:   fmt.Sprintf("racelog.public.live.speedmap.%s", pd.Event.Key),
			handler: pm.speedmapMessageHandler(pd),
		}, {
			topic:   fmt.Sprintf("racelog.public.live.cardata.%s", pd.Event.Key),
			handler: pm.carMessageHandler(pd),
		},
	}

	for _, t := range topics {
		log.Debug("Subscribing", log.String("topic", t.topic))
		if err := cli.Subscribe(t.topic, t.handler, wamp.Dict{}); err != nil {
			log.Error("Could not subscribe",
				log.String("topic", t.topic),
				log.ErrorField(err))
			return
		}
	}

	log.Debug("Creating guard", log.String("eventKey", pd.Event.Key))
	go func() {
		done := <-ctx.Done()
		log.Debug("ctx.Done() received.", log.Any("ctxDone", done))
		for _, t := range topics {
			if err := cli.Unsubscribe(t.topic); err != nil {
				log.Error("Error unsubscribing",
					log.String("topic", t.topic), log.ErrorField(err))
			} else {
				log.Debug("Unsubscribed", log.String("topic", t.topic))
			}
		}
	}()
}

//nolint:lll,dupl //by design
func (pm *ProviderManager) stateMessageHandler(pd *service.ProviderData) client.EventHandler {
	return func(event *wamp.Event) {
		log.Debug("received message", log.Any("msg", event.Arguments))
		stateData, err := prepareStateData(event)
		if err != nil {
			log.Error("Error preparing stateData",
				log.ErrorField(err))
			return
		}
		if err := pm.stateService.AddState(&model.DbState{
			EventID: pd.Event.ID,
			Data:    *stateData,
		}); err != nil {
			log.Error("Error storing stateData",
				log.ErrorField(err))
			return
		}
	}
}

//nolint:lll,dupl //by design
func (pm *ProviderManager) speedmapMessageHandler(pd *service.ProviderData) client.EventHandler {
	return func(event *wamp.Event) {
		log.Debug("received message", log.Any("msg", event.Arguments))
		speedmapData, err := prepareSpeedmapData(event)
		if err != nil {
			log.Error("Error preparing speedmapData",
				log.ErrorField(err))
			return
		}
		if err := pm.speedmapService.AddSpeedmap(&model.DbSpeedmap{
			EventID: pd.Event.ID,
			Data:    *speedmapData,
		}); err != nil {
			log.Error("Error storing speedmapData",
				log.ErrorField(err))
			return
		}
	}
}

//nolint:lll,dupl //by design
func (pm *ProviderManager) carMessageHandler(pd *service.ProviderData) client.EventHandler {
	return func(event *wamp.Event) {
		log.Debug("received message", log.Any("msg", event.Arguments))
		carData, err := prepareCarData(event)
		if err != nil {
			log.Error("Error preparing carData",
				log.ErrorField(err))
			return
		}
		if err := pm.carService.AddCar(&model.DbCar{
			EventID: pd.Event.ID,
			Data:    *carData,
		}); err != nil {
			log.Error("Error storing carData",
				log.ErrorField(err))
			return
		}
	}
}

//nolint:whitespace //can't make both editor and linter happy
func prepareStateData(event *wamp.Event) (
	*model.StateData,
	error,
) {
	if len(event.Arguments) != 1 {
		return nil, fmt.Errorf("need exact 1 argument in request")
	}
	if _, ok := event.Arguments[0].(map[string]interface{}); ok {
		wDict, _ := wamp.AsDict(event.Arguments[0])
		jsonData, _ := json.Marshal(wDict)
		var ret model.StateData
		err := json.Unmarshal(jsonData, &ret)
		return &ret, err
	}
	return nil, fmt.Errorf("invalid data in message")
}

//nolint:whitespace //can't make both editor and linter happy
func prepareSpeedmapData(event *wamp.Event) (
	*model.SpeedmapData,
	error,
) {
	if len(event.Arguments) != 1 {
		return nil, fmt.Errorf("need exact 1 argument in request")
	}
	if _, ok := event.Arguments[0].(map[string]interface{}); ok {
		wDict, _ := wamp.AsDict(event.Arguments[0])
		jsonData, _ := json.Marshal(wDict)
		var ret model.SpeedmapData
		err := json.Unmarshal(jsonData, &ret)
		return &ret, err
	}
	return nil, fmt.Errorf("invalid data in message")
}

//nolint:whitespace //can't make both editor and linter happy
func prepareCarData(event *wamp.Event) (
	*model.CarData,
	error,
) {
	if len(event.Arguments) != 1 {
		return nil, fmt.Errorf("need exact 1 argument in request")
	}
	if _, ok := event.Arguments[0].(map[string]interface{}); ok {
		wDict, _ := wamp.AsDict(event.Arguments[0])
		jsonData, _ := json.Marshal(wDict)
		var ret model.CarData
		err := json.Unmarshal(jsonData, &ret)
		return &ret, err
	}
	return nil, fmt.Errorf("invalid data in message")
}

func (pm *ProviderManager) handleRemoveProvider() error {
	return pm.wampClient.Register("racelog.dataprovider.remove_provider",
		func(ctx context.Context, inv *wamp.Invocation) client.InvokeResult {
			log.Info("Received remove provider request")

			req, err := utils.ExtractEventKey(inv)
			if err != nil {
				return client.InvokeResult{
					Args: wamp.List{"invalid remove registration request"},
				}
			}
			log.Debug("received data", log.String("eventKey", req))
			if pd, ok := pm.pService.Lookup[req]; ok {
				log.Debug("Calling cancel func", log.String("eventKey", req))
				pd.ActiveClient.CancelFunc()
				pm.publishRemovedProvider(pd)
				delete(pm.pService.Lookup, req)
				return client.InvokeResult{
					Args: wamp.List{fmt.Sprintf("provider for eventKey %s removed", req)},
				}
			} else {
				return client.InvokeResult{
					Args: wamp.List{fmt.Sprintf("no provider for eventKey %s", req)},
				}
			}
		}, wamp.Dict{})
}

func (pm *ProviderManager) handleEventExtraData() error {
	return pm.wampClient.Register("racelog.dataprovider.store_event_extra_data",
		func(ctx context.Context, inv *wamp.Invocation) client.InvokeResult {
			log.Info("Received extra event data ", log.Any("inv", inv))
			eventKey, extData, err := extractExtraData(inv)
			if err != nil {
				return client.InvokeResult{
					Args: wamp.List{"invalid remove registration request"},
				}
			}
			if pd, ok := pm.pService.Lookup[*eventKey]; ok {
				if err := pm.pService.StoreEventExtra(&model.DbEventExtra{
					EventID: pd.Event.ID,
					Data:    *extData,
				}); err != nil {
					log.Error("store extra data",
						log.ErrorField(err),
						log.Any("extraData", inv.Arguments))
					return client.InvokeResult{
						Args: wamp.List{"error processing extra data"},
					}
				}
			}

			return client.InvokeResult{}
		}, wamp.Dict{})
}

//nolint:whitespace //can't make all linters happy
func extractRegisterRequest(inv *wamp.Invocation) (
	*service.RegisterEventRequest,
	error,
) {
	if len(inv.Arguments) != 1 {
		return nil, fmt.Errorf("need exact 1 argument in request")
	}
	if _, ok := inv.Arguments[0].(map[string]interface{}); ok {
		wDict, _ := wamp.AsDict(inv.Arguments[0])
		jsonData, _ := json.Marshal(wDict)
		var ret service.RegisterEventRequest
		err := json.Unmarshal(jsonData, &ret)
		return &ret, err
	}
	return nil, fmt.Errorf("invalid request in message")
}

//nolint:whitespace //can't make all linters happy
func extractExtraData(inv *wamp.Invocation) (*string, *model.ExtraInfo, error) {
	if len(inv.Arguments) != 2 {
		return nil, nil, fmt.Errorf("need exact 2 argument in request")
	}
	// 0 - eventKey
	var eventKey string
	if _, ok := inv.Arguments[0].(string); ok {
		if eventKey, ok = wamp.AsString(inv.Arguments[0]); !ok {
			return nil, nil, fmt.Errorf("cannot extract event key")
		}
	}
	// 1 - trackInfo
	if _, ok := inv.Arguments[1].(map[string]interface{}); ok {
		wDict, _ := wamp.AsDict(inv.Arguments[1])

		jsonData, _ := json.Marshal(wDict)
		var ret model.ExtraInfo
		err := json.Unmarshal(jsonData, &ret)
		return &eventKey, &ret, err

	}
	return nil, nil, fmt.Errorf("invalid request in message")
}

func createProviderResponseData(data *service.ProviderData) *ProviderResponseData {
	return &ProviderResponseData{
		EventKey:   data.Event.Key,
		Info:       data.Event.Data.Info,
		RecordDate: data.Registered,
		Manifests:  data.Event.Data.Manifests,
		DbId:       data.Event.ID,
	}
}
