package public

import (
	"context"
	"fmt"
	"time"

	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
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
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/service"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

type (
	ProviderLookupFunc func(key string) *service.ProviderData
	PublicManager      struct {
		endpoints    []endpointHandler
		wampClient   *client.Client
		providerFunc ProviderLookupFunc
	}
)

type endpointParam struct {
	name     string
	pool     *pgxpool.Pool
	recorder metric.Float64Histogram
}

type endpointHandler struct {
	name    string
	handler func(*endpointParam) client.InvocationHandler
	// metricName string
	metricRecorder metric.Float64Histogram
}

//nolint:whitespace,unparam //ok
func prepareHandler(
	name,
	metricName,
	description string,
	handler func(*endpointParam) client.InvocationHandler,
) endpointHandler {
	if metricName == "" {
		metricName = name
	}
	if description == "" {
		description = name
	}
	recorder, _ := meter.Float64Histogram(metricName,
		metric.WithDescription(description),
		metric.WithUnit("s"))
	return endpointHandler{
		name:           name,
		handler:        handler,
		metricRecorder: recorder,
	}
}

var (
	meter  = otel.Meter("public-endpoints")
	tracer = otel.Tracer("public-endpoints")
)

//nolint:whitespace,gocritic //can't make both the linter and editor happy :(
func InitPublicEndpoints(
	pool *pgxpool.Pool,
	providerLookup ProviderLookupFunc,
	logger *log.Logger,
) (*PublicManager, error) {
	wampClient, err := utils.NewClient(utils.WithClientLogging(logger))
	if err != nil {
		log.Fatal("Could not connect wamp client", log.ErrorField(err))
	}
	ret := PublicManager{
		wampClient:   wampClient,
		providerFunc: providerLookup,
		endpoints: []endpointHandler{
			prepareHandler("get_events", "", "get list of events", getEventList),
			prepareHandler("get_event_info", "", "", getEventInfoByIdHandler),
			prepareHandler("get_event_info_by_key", "", "", getEventInfoByKeyHandler),
			prepareHandler("get_event_cars", "", "", getEventCarsById),
			prepareHandler("get_event_cars_by_key", "", "", getEventCarsByKey),
			prepareHandler("get_event_speedmap", "", "", getSpeemapById),
			prepareHandler("get_event_speedmap_by_key", "", "", getSpeemapByKey),
			prepareHandler("get_track_info", "", "", getTrackInfoByIdHandler),
			prepareHandler("archive.get_event_analysis", "", "", getEventAnalysisById),
			prepareHandler("archive.avglap_over_time", "", "", getAvgLapOverTime),
			prepareHandler("archive.state.delta", "", "", getStatesWithDiff),
			prepareHandler("archive.speedmap", "", "", getArchivedSpeedmap),
			prepareHandler("get_version", "", "", getVersion),
		},
	}
	for _, endpoint := range ret.endpoints {
		name := fmt.Sprintf("racelog.public.%s", endpoint.name)

		if err := wampClient.Register(name,
			endpoint.handler(
				&endpointParam{name, pool, endpoint.metricRecorder}),
			wamp.Dict{}); err != nil {
			log.Error("Register", zap.String("endpoint", name), log.ErrorField(err))
		}

	}

	name := "racelog.public.live.get_event_analysis_by_key"
	if err := wampClient.Register(name,
		getLiveEventAnalysisByKey(ret.providerFunc), wamp.Dict{}); err != nil {
		log.Error("Register", zap.String("endpoint", name), log.ErrorField(err))
	}
	return &ret, nil
}

//nolint:gocritic //by design
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

func getVersion(param *endpointParam) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		log.Info("get_version")
		return client.InvokeResult{Args: wamp.List{
			map[string]string{"ownVersion": version.Version},
		}}
	}
}

func getLiveEventAnalysisByKey(lookupFunc ProviderLookupFunc) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		log.Info("live.get_event_analysis_by_key")
		eventKey, err := utils.ExtractEventKey(i)
		if err != nil {
			log.Error("Error extracting event key", log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{err}}
		}
		pd := lookupFunc(eventKey)
		return client.InvokeResult{Kwargs: wamp.Dict{
			"processedData": pd.Processor.GetData(),
			"manifests":     pd.Processor.Manifests,
		}}
	}
}

func getEventInfoByIdHandler(ep *endpointParam) client.InvocationHandler {
	return getGeneric(ep, utils.ExtractId, event.LoadById)
}

func getEventInfoByKeyHandler(ep *endpointParam) client.InvocationHandler {
	return getGeneric(ep, utils.ExtractEventKey, event.LoadByKey)
}

func getEventCarsById(ep *endpointParam) client.InvocationHandler {
	return getGenericWithConvert(
		ep, utils.ExtractId, car.LoadLatestByEventId,
		func(entry *model.DbCar) *model.CarData { return &entry.Data })
}

func getEventCarsByKey(ep *endpointParam) client.InvocationHandler {
	return getGenericWithConvert(
		ep, utils.ExtractEventKey, car.LoadLatestByEventKey,
		func(entry *model.DbCar) *model.CarData { return &entry.Data })
}

func getSpeemapById(ep *endpointParam) client.InvocationHandler {
	return getGenericWithConvert(
		ep, utils.ExtractId, speedmap.LoadLatestByEventId,
		func(entry *model.DbSpeedmap) *model.SpeedmapData { return &entry.Data })
}

func getSpeemapByKey(ep *endpointParam) client.InvocationHandler {
	return getGenericWithConvert(
		ep, utils.ExtractEventKey, speedmap.LoadLatestByEventKey,
		func(entry *model.DbSpeedmap) *model.SpeedmapData { return &entry.Data })
}

func getTrackInfoByIdHandler(ep *endpointParam) client.InvocationHandler {
	return getGenericWithConvert(
		ep, utils.ExtractId, track.LoadById,
		func(entry *model.DbTrack) *model.TrackInfo { return &entry.Data })
}

func getEventAnalysisById(ep *endpointParam) client.InvocationHandler {
	ctx := context.Background()
	start := time.Now()
	_, span := tracer.Start(ctx, "get-event-analysis")
	defer span.End()
	defer func() {
		ep.recorder.Record(ctx, time.Since(start).Seconds())
	}()

	return getGenericWithConvert(
		ep, utils.ExtractId, analysis.LoadByEventId,
		func(entry *model.DbAnalysis) *model.AnalysisDataGeneric { return &entry.Data })
}

func getEventList(ep *endpointParam) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		start := time.Now()
		myCtx, span := tracer.Start(ctx, "get-event-list")
		defer span.End()
		defer func() {
			ep.recorder.Record(myCtx, time.Since(start).Seconds())
		}()

		data, err := event.LoadAll(myCtx, ep.pool)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getArchivedSpeedmap(ep *endpointParam) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		start := time.Now()
		ctx, span := tracer.Start(ctx, "archived speedmap")
		defer span.End()
		defer func() {
			ep.recorder.Record(ctx, time.Since(start).Seconds())
		}()

		param, err := utils.ExtractRangeTuple(i)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		data, err := speedmap.LoadRange(ctx, ep.pool, param.EventID, param.TsBegin, param.Num)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getAvgLapOverTime(ep *endpointParam) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		start := time.Now()
		ctx, span := tracer.Start(ctx, "avg-lap-over-time")
		defer span.End()
		defer func() {
			ep.recorder.Record(ctx, time.Since(start).Seconds())
		}()

		param, err := utils.ExtractParamAvgLap(i)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		data, err := speedmap.LoadAvgLapOverTime(
			ctx,
			ep.pool,
			param.EventID,
			param.IntervalSecs)
		if err != nil {
			return client.InvokeResult{Args: wamp.List{err}}
		}
		return client.InvokeResult{Args: wamp.List{data}}
	}
}

func getStatesWithDiff(ep *endpointParam) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		start := time.Now()
		ctx, span := tracer.Start(ctx, "state-with-diff")
		defer span.End()
		defer func() {
			ep.recorder.Record(ctx, time.Since(start).Seconds())
		}()

		param, err := utils.ExtractRangeTuple(i)
		if err != nil {
			log.Error("Error extracting range tuple", log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{err}}
		}
		first, delta, err := state.LoadByEventIdWithDelta(
			ctx, ep.pool, param.EventID, param.TsBegin, param.Num)
		if err != nil {
			log.Error("Error loading states", log.ErrorField(err))
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

//nolint:lll //better readability
type (
	keyExtractor[K any]         func(i *wamp.Invocation) (K, error)
	loader[K any, T any]        func(ctx context.Context, c repository.Querier, key K) (*T, error)
	postProcessor[T any, F any] func(entry *T) *F
)

//nolint:whitespace // can't make editor and linters both happy
func getGeneric[K, T any](
	ep *endpointParam,
	extractor keyExtractor[K],
	loaderFunc loader[K, T],
) client.InvocationHandler {
	return getGenericWithConvert(ep,
		extractor,
		loaderFunc,
		func(entry *T) *T { return entry })
}

//nolint:whitespace // can't make editor and linters both happy
func getGenericWithConvert[K, T, R any](
	ep *endpointParam,
	extractor keyExtractor[K],
	loaderFunc loader[K, T],
	postProc postProcessor[T, R],
) client.InvocationHandler {
	return func(ctx context.Context, i *wamp.Invocation) client.InvokeResult {
		start := time.Now()
		ctx, span := tracer.Start(ctx, ep.name)
		defer span.End()
		defer func() {
			ep.recorder.Record(ctx, time.Since(start).Seconds())
		}()
		key, err := extractor(i)
		if err != nil {
			log.Error("Error extracting key", zap.Any("key", key), log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{}}
		}
		e, err := loaderFunc(ctx, ep.pool, key)
		if err != nil {
			log.Error("Load data", zap.Any("key", key), log.ErrorField(err))
			return client.InvokeResult{Args: wamp.List{}}
		}
		result := postProc(e)

		return client.InvokeResult{Args: wamp.List{result}}
	}
}
