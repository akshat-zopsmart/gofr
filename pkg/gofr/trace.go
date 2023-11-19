package gofr

import (
	"context"
	"gofr.dev/pkg"
	"strconv"
	"strings"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

const (
	ZIPKIN = "zipkin"
	GCP    = "gcp"
)

func tracerProvider(c Config, logger log.Logger) error {

	if tp, err := getTraceProvider(c, logger); err == nil {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	} else {
		return err
	}

	return nil
}

func getTraceProvider(c Config, logger log.Logger) (tp *trace.TracerProvider, err error) {
	exporterName := strings.ToLower(c.Get("TRACER_EXPORTER"))

	if exporterName == "" {
		logger.Warnf("exporter not defined, tracing would not be enabled")
		return
	}

	var spanExporter trace.SpanExporter

	switch exporterName {
	case ZIPKIN:
		spanExporter, err = getZipkinExporter(c)
	case GCP:
		spanExporter, err = getGCPExporter(c)
	default:
		err = errors.Error("invalid exporter")
	}

	if err != nil {
		return
	}

	r, err := resource.New(context.Background(), resource.WithAttributes(getResourceAttributes(c)...))
	if err != nil {
		return
	}

	sampler := getSampler(c, logger)

	tp = trace.NewTracerProvider(trace.WithSampler(sampler), trace.WithBatcher(spanExporter), trace.WithResource(r))
	return
}

func getZipkinExporter(c Config) (*zipkin.Exporter, error) {
	url := c.Get("TRACER_URL") + "/api/v2/spans"

	exporter, err := zipkin.New(url)
	if err != nil {
		return nil, err
	}

	return exporter, nil
}

func getGCPExporter(c Config) (*cloudtrace.Exporter, error) {

	projectID := c.Get("GCP_PROJECT_ID")
	if projectID == "" {
		return nil, errors.Error("Require GCP_PROJECT_ID env to be set")
	}

	exporter, err := cloudtrace.New(cloudtrace.WithProjectID(projectID))
	if err != nil {
		return nil, err
	}

	return exporter, nil
}

func getSampler(c Config, logger log.Logger) trace.Sampler {

	// if isAlwaysSample is set true for any service, it will sample all the trace
	// else it will be sampled based on parent of the span.
	isAlwaysSample := getBoolOrDefault(c, logger, "TRACER_ALWAYS_SAMPLE", false)

	var sampler trace.Sampler

	if isAlwaysSample {
		sampler = trace.AlwaysSample()
	} else {
		tracerRatio := getFloat64OrDefault(c, logger, "TRACER_RATIO", 0.1)
		sampler = trace.ParentBased(trace.TraceIDRatioBased(tracerRatio))
	}
	return sampler
}

func getResourceAttributes(c Config) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
		attribute.String(string(semconv.TelemetrySDKVersionKey), c.GetOrDefault("APP_VERSION", pkg.DefaultAppVersion)),
		attribute.String(string(semconv.ServiceNameKey), c.GetOrDefault("APP_NAME", pkg.DefaultAppName)),
	}

	return attributes
}

// getBoolOrDefault returns the bool value of the given config
// or default value in case the config is missing/invalid.
//
//	This function should be moved to config package eventually to avoid redundancy
func getBoolOrDefault(c Config, logger log.Logger, key string, defaultValue bool) bool {
	val := c.Get(key)

	if val == "" {
		logger.Warnf("%s is not set.'%s' will be used by default", key, strconv.FormatBool(defaultValue))
		return defaultValue
	}

	if parsedVal, err := strconv.ParseBool(val); err == nil {
		return parsedVal
	} else {
		logger.Warnf("%s set in %s is not parseable into float.'%s' will be used by default", val, key, strconv.FormatBool(defaultValue))
		return defaultValue
	}

}

// getFloat64OrDefault returns the float value of the given config
// or default value in case the config is missing/invalid.
//
//	This function should be moved to config package eventually to avoid redundancy
func getFloat64OrDefault(c Config, logger log.Logger, key string, defaultValue float64) float64 {
	val := c.Get(key)

	if val == "" {
		logger.Warnf("%s is not set.'%s' will be used by default", key, defaultValue)
		return defaultValue
	}

	if parsedVal, err := strconv.ParseFloat(val, 64); err == nil {
		return parsedVal
	} else {
		logger.Warnf("%s set in %s is not parseable into float.'%s' will be used by default", val, key, defaultValue)
		return defaultValue
	}

}
