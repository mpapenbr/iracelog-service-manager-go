package public

import (
	"context"
	"fmt"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/endpoints/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/analysis"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/car"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/event"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/speedmap"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/state"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/repository/track"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

type PublicManager struct {
	endpoints  []endpointHandler
	wampClient *client.Client
}

type endpointHandler struct {
	name    string
	handler func(*pgxpool.Pool) client.InvocationHandler
}

func InitPublicEndpoints(pool *pgxpool.Pool) (*PublicManager, error) {
	wampClient, err := utils.NewClient()
	if err != nil {
		log.Fatal("Could not connect wamp client", log.ErrorField(err))
	}
	ret := PublicManager{
		wampClient: wampClient,
		endpoints: []endpointHandler{
			{name: "get_events", handler: getEventList},
			{name: "get_event_info_by_key", handler: getEventInfoByKeyHandler},
			{name: "get_event_info", handler: getEventInfoByIdHandler},
			{name: "get_event_cars", handler: getEventCarsById},
			{name: "get_event_cars_by_key", handler: getEventCarsByKey},
			{name: "get_event_speedmap", handler: getSpeemapById},
			{name: "get_event_speedmap_by_key", handler: getSpeemapByKey},
			{name: "get_track_info", handler: getTrackInfoByIdHandler},
			{name: "archive.get_event_analysis", handler: getEventAnalysisById},
			{name: "archive.avglap_over_time", handler: getAvgLapOverTime},
			{name: "archive.state.delta", handler: getStatesWithDiff},
			{name: "archive.speedmap", handler: getArchivedSpeedmap},
			{name: "get_version", handler: getVersion},
		},
	}
	for _, endpoint := range ret.endpoints {
		name := fmt.Sprintf("racelog.public.%s", endpoint.name)
		if err := wampClient.Register(name, endpoint.handler(pool), wamp.Dict{}); err != nil {
			log.Error("Register", zap.String("endpoint", name), log.ErrorField(err))
		}
	}
	return &ret, nil
}

func (pub *PublicManager) Shutdown() {
	for _, endpoint := range pub.endpoints {
		name := fmt.Sprintf("racelog.public.%s", endpoint.name)
		log.Info("Unregistering ", zap.String("endpoint", name))
		err := pub.wampClient.Unregister(name)
		if err != nil {
			log.Error("Failed to unregister procedure:", log.ErrorField(err))
		}
	}
}

func getVersion(pool *pgxpool.Pool) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		log.Info("get_version")
		return client.InvokeResult{Args: wamp.List{
			map[string]string{"ownVersion": version.Version},
		}}
	}
}

func getEventInfoByIdHandler(pool *pgxpool.Pool) client.InvocationHandler {
	return getGeneric(pool, utils.ExtractId, event.LoadById)
}

func getEventInfoByKeyHandler(pool *pgxpool.Pool) client.InvocationHandler {
	return getGeneric(pool, utils.ExtractEventKey, event.LoadByKey)
}

func getEventCarsById(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractId, car.LoadLatestByEventId,
		func(entry *model.DbCar) *model.CarData { return &entry.Data })
}

func getEventCarsByKey(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractEventKey, car.LoadLatestByEventKey,
		func(entry *model.DbCar) *model.CarData { return &entry.Data })
}

func getSpeemapById(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractId, speedmap.LoadLatestByEventId,
		func(entry *model.DbSpeedmap) *model.SpeedmapData { return &entry.Data })
}

func getSpeemapByKey(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractEventKey, speedmap.LoadLatestByEventKey,
		func(entry *model.DbSpeedmap) *model.SpeedmapData { return &entry.Data })
}

func getTrackInfoByIdHandler(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractId, track.LoadById,
		func(entry *model.DbTrack) *model.TrackInfo { return &entry.Data })
}

func getEventAnalysisById(pool *pgxpool.Pool) client.InvocationHandler {
	return getGenericWithConvert(
		pool, utils.ExtractId, analysis.LoadByEventId,
		func(entry *model.DbAnalysis) *model.AnalysisData { return &entry.Data })
}

func getEventList(pool *pgxpool.Pool) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		data, err := event.LoadAll(pool)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getArchivedSpeedmap(pool *pgxpool.Pool) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		param, err := utils.ExtractRangeTuple(i)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		data, err := speedmap.LoadRange(pool, param.EventID, param.TsBegin, param.Num)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getAvgLapOverTime(pool *pgxpool.Pool) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		param, err := utils.ExtractParamAvgLap(i)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		data, err := speedmap.LoadAvgLapOverTime(pool, param.EventID, param.IntervalSecs)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getStatesWithDiff(pool *pgxpool.Pool) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		param, err := utils.ExtractRangeTuple(i)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		first, delta, err := state.LoadByEventIdWithDelta(
			pool, param.EventID, param.TsBegin, param.Num)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}

		if first == nil {
			return client.InvokeResult{Args: wamp.List{}}
		}

		combined := make([]any, 1+len(delta))
		combined[0] = first
		for i, v := range delta {
			combined[i+1] = v
		}
		return client.InvokeResult{Args: wamp.List{combined}}
	}
}

type (
	keyExtractor[K any]         func(i *wamp.Invocation) (K, error)
	loader[K any, T any]        func(c repository.Querier, key K) (*T, error)
	postProcessor[T any, F any] func(entry *T) *F
)

//nolint:whitespace // can't make editor and linters both happy
func getGeneric[K, T any](
	pool *pgxpool.Pool,
	extractor keyExtractor[K],
	loaderFunc loader[K, T],
) client.InvocationHandler {
	return getGenericWithConvert(pool,
		extractor,
		loaderFunc,
		func(entry *T) *T { return entry })
}

//nolint:whitespace // can't make editor and linters both happy
func getGenericWithConvert[K, T, R any](
	pool *pgxpool.Pool,
	extractor keyExtractor[K],
	loaderFunc loader[K, T],
	postProc postProcessor[T, R],
) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		key, err := extractor(i)
		if err != nil {
			log.Error("Error extracting key", zap.Any("key", key), log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{}}
		}
		e, err := loaderFunc(pool, key)
		if err != nil {
			log.Error("Load data", zap.Any("key", key), log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{}}
		}
		result := postProc(e)

		return client.InvokeResult{Args: wamp.List{result}}
	}
}
