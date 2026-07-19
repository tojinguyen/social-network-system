package metrics

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// InitMetrics initializes OpenTelemetry Metrics with Prometheus exporter.
// It returns the prometheus http.Handler and a shutdown function.
func InitMetrics(ctx context.Context, serviceName string) (http.Handler, func(), error) {
	if os.Getenv("OTEL_METRICS_ENABLED") != "true" {
		log.Printf("OTel Metrics is disabled for %s", serviceName)
		return nil, func() {}, nil
	}

	registry := prometheus.NewRegistry()

	// Register Go runtime and process metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Register OpenTelemetry exporter to the custom registry automatically
	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, nil, err
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)

	// Set as global MeterProvider (optional but good practice)
	// otel.SetMeterProvider(provider)

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Printf("Failed to shutdown MeterProvider: %v", err)
		}
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	log.Printf("OTel Metrics (Prometheus exporter) initialized for %s", serviceName)
	return handler, shutdown, nil
}
