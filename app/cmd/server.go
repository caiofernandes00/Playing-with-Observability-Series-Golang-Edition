package main

import (
	"context"
	"log"
	"net/http"
	"observability-series-golang-edition/app/infrastructure/api"
	"observability-series-golang-edition/app/infrastructure/circuitbreaker"
	"observability-series-golang-edition/app/infrastructure/metrics"
	"observability-series-golang-edition/app/util"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/net/http2"
)

var (
	e           *echo.Echo
	http2Server *http2.Server
	config      *util.Config
)

func init() {
	e = echo.New()
	loadEnv()
	cb := circuitbreaker.NewCircuitBreaker(config.CircuitBreakerInterval, config.CircuitBreakerThreshold)
	metrics.MetricsRegister()
	api.MiddlewareRegister(e, cb)
	api.RoutesRegister(e)
	loadHttp2Server()
}

func main() {
	go func() {
		if err := e.StartH2CServer(":"+config.Port, http2Server); err != http.ErrServerClosed {
			log.Fatal(err)
		}

		log.Println("Server is running on port " + config.Port)
	}()

	gracefulShutdown()
}

func gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func loadHttp2Server() {
	http2Server = &http2.Server{
		MaxConcurrentStreams: config.MaxConcurrentStreams,
		MaxReadFrameSize:     config.MaxReadFrameSize,
		IdleTimeout:          config.IdleTimeout * time.Second,
	}
}

func loadEnv() {
	config = util.NewConfig()
	path, err := getRootFile()

	if err == nil {
		config.LoadEnvFile(path)
		return
	}

	config.LoadEnv()

	return
}

func getRootFile() (ex string, err error) {
	ex, _ = os.Getwd()
	_, err = os.Stat(filepath.Join(ex, "app.env"))

	if err != nil {
		ex = filepath.Join(ex, "../../")
		_, err = os.Stat(filepath.Join(ex, "app.env"))

		if err != nil {
			log.Println("No env file provided, using only env variables")
		}
	}

	return
}
