package gateway

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dewani12/photon/pkg/logger"
	"github.com/dewani12/photon/pkg/metrics"
	"github.com/dewani12/photon/pkg/trace"
	"github.com/dewani12/photon/pkg/config"
	"github.com/joho/godotenv"
)

type Config struct{
	UpstreamURL string
	APIKey string
	Port string
	ExporterURL string
	ServiceName string
}

type Gateway struct{
	config Config
	server *http.Server
	client *http.Client
	exporter *trace.OTLPExporter
}

func DefaultConfig()Config{
	godotenv.Load()
	return Config{
		UpstreamURL: os.Getenv("UPSTREAM_URL"),
		APIKey: os.Getenv("API_KEY"),
		Port: config.GetEnv("PORT",":5000"),
		ExporterURL: os.Getenv("EXPORTER_URL"),
		ServiceName: "llm-gateway",
	}
}

func New(cfg Config)*Gateway{
	return &Gateway{
		config: cfg,
		client: &http.Client{
			Timeout: 60*time.Second,
		},
	}
}

func (g* Gateway)Start(){
	logger.Init()
	Init() //metrics

	g.exporter=trace.NewExporter(g.config.ExporterURL)
	g.exporter.Start()
	trace.GlobalExporter=g.exporter

	trace.ServiceName=g.config.ServiceName

	http.Handle("/v1/chat/completions",trace.Middleware(http.HandlerFunc(g.ChatHandler)))

	http.Handle("/metrics",metrics.Default.Handler())

	g.server=&http.Server{
		Addr: ":"+g.config.Port,
	}

	go func(){
		logger.L.Info("llm gateway started","port",g.config.Port)
		g.server.ListenAndServe()
	}()

	quit:=make(chan os.Signal,1)
	signal.Notify(quit,syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.L.Info("llm gateway shutting down")

	g.exporter.Shutdown(context.Background())
	g.server.Shutdown(context.Background())
}


