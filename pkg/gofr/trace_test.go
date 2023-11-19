package gofr

import (
	"bytes"
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/trace"
	"gofr.dev/pkg"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func Test_tracerProvider(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	testCases := []struct {
		configData map[string]string
		expected   *trace.TracerProvider
		err        bool
	}{
		{map[string]string{"TRACER_EXPORTER": ZIPKIN}, nil, true},
		{map[string]string{"TRACER_EXPORTER": ZIPKIN, "TRACER_URL": "http://localhost:9411"}, nil, false},
		{map[string]string{"TRACER_EXPORTER": GCP}, nil, true},
		{map[string]string{"TRACER_EXPORTER": ""}, nil, false},
	}
	for _, testCase := range testCases {
		c := &config.MockConfig{Data: testCase.configData}
		_, err := getTraceProvider(c, logger)
		assert.Equal(t, err != nil, testCase.err)
	}
}
func Test_getResourceAttributes(t *testing.T) {
	testCases := []struct {
		configData map[string]string
		attributes []attribute.KeyValue
	}{
		{map[string]string{"APP_VERSION": "1", "APP_NAME": "Sample"}, []attribute.KeyValue{
			attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
			attribute.String(string(semconv.TelemetrySDKVersionKey), "1"),
			attribute.String(string(semconv.ServiceNameKey), "Sample"),
		}},
		{map[string]string{"APP_VERSION": "", "APP_NAME": "Sample"}, []attribute.KeyValue{
			attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
			attribute.String(string(semconv.TelemetrySDKVersionKey), pkg.DefaultAppVersion),
			attribute.String(string(semconv.ServiceNameKey), "Sample"),
		}},
		{map[string]string{"APP_VERSION": "1", "APP_NAME": ""}, []attribute.KeyValue{
			attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
			attribute.String(string(semconv.TelemetrySDKVersionKey), "1"),
			attribute.String(string(semconv.ServiceNameKey), pkg.DefaultAppName),
		}},
		{map[string]string{}, []attribute.KeyValue{
			attribute.String(string(semconv.TelemetrySDKLanguageKey), "go"),
			attribute.String(string(semconv.TelemetrySDKVersionKey), pkg.DefaultAppVersion),
			attribute.String(string(semconv.ServiceNameKey), pkg.DefaultAppName),
		}},
	}

	for _, testCase := range testCases {
		mockConfig := &config.MockConfig{Data: testCase.configData}
		assert.Equal(t, testCase.attributes, getResourceAttributes(mockConfig))
	}
}

func Test_getSampler(t *testing.T) {
	testCases := []struct {
		configData   map[string]string
		sampler      trace.Sampler
		loggerString bool
	}{
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "", "TRACER_RATIO": ""}, trace.ParentBased(trace.TraceIDRatioBased(0.1)), true},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "", "TRACER_RATIO": "0.3"}, trace.ParentBased(trace.TraceIDRatioBased(0.3)), true},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "", "TRACER_RATIO": "hello"}, trace.ParentBased(trace.TraceIDRatioBased(0.1)), true},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "hello", "TRACER_RATIO": "hello"}, trace.ParentBased(trace.TraceIDRatioBased(0.1)), true},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "true", "TRACER_RATIO": "0.3"}, trace.AlwaysSample(), false},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "true", "TRACER_RATIO": "hello"}, trace.AlwaysSample(), false},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "true", "TRACER_RATIO": ""}, trace.AlwaysSample(), false},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "false", "TRACER_RATIO": "0.2"}, trace.ParentBased(trace.TraceIDRatioBased(0.2)), false},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "false", "TRACER_RATIO": "hello"}, trace.ParentBased(trace.TraceIDRatioBased(0.1)), true},
		{map[string]string{"TRACER_ALWAYS_SAMPLE": "false", "TRACER_RATIO": ""}, trace.ParentBased(trace.TraceIDRatioBased(0.1)), true},
	}

	for _, testCase := range testCases {
		mockConfig := &config.MockConfig{Data: testCase.configData}
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)
		assert.Equal(t, testCase.sampler, getSampler(mockConfig, logger))
		if testCase.loggerString {
			assert.NotEmptyf(t, b.String(), "Log empty")
		}
		// We're not validating for empty logger scenarios as the config function
		// may have logs for success scenario as well
	}
}

func Test_getZipkinExporter(t *testing.T) {
	validExporter, err := zipkin.New("http://localhost/9411" + "/api/v2/spans")
	if err != nil {
		t.Logf("Failed to setup test %v", err)
	}

	testCases := []struct {
		configData map[string]string
		exporter   *zipkin.Exporter
		err        bool
	}{
		{map[string]string{"TRACER_URL": ""}, nil, true},
		{map[string]string{"TRACER_URL": "http://localhost/9411"}, validExporter, false},
	}

	for _, testCase := range testCases {
		mockConfig := &config.MockConfig{Data: testCase.configData}
		result, err := getZipkinExporter(mockConfig)
		assert.Equal(t, testCase.exporter, result)
		assert.Equal(t, err != nil, testCase.err)
	}
}

func Test_getGCPExporter(t *testing.T) {

	validExporter, err := cloudtrace.New(cloudtrace.WithProjectID("12345678"))
	if err != nil {
		t.Fatalf("Failed to setup test %v", err)
	}

	testCases := []struct {
		configData map[string]string
		exporter   *cloudtrace.Exporter
		err        bool
	}{
		{map[string]string{"GCP_PROJECT_ID": ""}, nil, true},
		{map[string]string{"GCP_PROJECT_ID": "12345678"}, validExporter, false},
	}

	for _, testCase := range testCases {
		mockConfig := &config.MockConfig{Data: testCase.configData}
		result, err := getGCPExporter(mockConfig)
		assert.Equal(t, testCase.exporter, result)
		assert.Equal(t, err != nil, testCase.err)
	}
}
