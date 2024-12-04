package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		oscall := <-ch
		log.Warn().Msgf("system call:%+v", oscall)
		cancel()
	}()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("logging-challange")),
	)

	if err != nil {
		log.Fatal().Msg("failed to initialize resource")
	}

	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal().Msg("failed to initialize prometheus exporter")
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
	)

	otel.SetMeterProvider(meterProvider)

	r := mux.NewRouter()
	r.Use(middleware)
	r.HandleFunc("/", handler)
	r.Handle("/metrics", promhttp.Handler())

	// start: set up any of your logger configuration here if necessary
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	lf, err := os.OpenFile(
		"logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open log file")
	}

	defer lf.Close()

	multiWriters := zerolog.MultiLevelWriter(os.Stdout, lf)
	log.Logger = zerolog.New(multiWriters).With().Timestamp().Logger()

	log.Info().Msg("starting server")
	// end: set up any of your logger configuration here

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("failed to listen and serve http server")
		}
	}()
	<-ctx.Done()

	if err := server.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("failed to shutdown http server gracefully")
	}
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := log.Logger.With().
			Str("request_id", uuid.New().String()).
			Str("url", r.URL.String()).
			Str("method", r.Method).
			Logger()

		ctx := log.WithContext(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	log := log.Ctx(ctx).With().Str("func", "handler").Logger()
	log.Debug().Msg("handler started")

	name := r.URL.Query().Get("name")
	res, err := greeting(ctx, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().Str("response", res).Msg("handler finished")
	w.Write([]byte(res))
}

func greeting(ctx context.Context, name string) (string, error) {
	log := log.Ctx(ctx)
	log.Debug().Str("func", "greeting").Str("name", name).Msg("greeting started")

	if len(name) < 5 {
		log.Warn().Msgf("name is too short: %s", name)
		return fmt.Sprintf("Hello %s! Your name is to short\n", name), nil
	}

	log.Info().Msgf("greeting %s", name)
	return fmt.Sprintf("Hi %s", name), nil
}
