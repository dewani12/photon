// Demo Server
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dewani12/photon/pkg/logger"
	"github.com/dewani12/photon/pkg/metrics"
	"github.com/dewani12/photon/pkg/trace"
	"github.com/joho/godotenv"
)

//inject logger into context
func observeMiddleware(next http.Handler)http.Handler{
	return trace.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l:=logger.L
		ctx:=logger.WithContext(l,r.Context())
	
	 	logger.WithSpan(ctx).Info("request started",
            "method", r.Method,
            "path", r.URL.Path,
        )

        next.ServeHTTP(w, r.WithContext(ctx))

        logger.WithSpan(ctx).Info("request completed",
            "method", r.Method,
            "path", r.URL.Path,
        )
	}))
}

func helloHandler(w http.ResponseWriter, r *http.Request){
	//trace for handler
	span,ctx:=trace.StartSpan(r.Context(),"hello-handler")
	l:=logger.WithSpan(ctx)
    l.Debug("hello handler called")

	span.SetAttribute("hello","world")
	defer span.End()
	fmt.Fprintln(w,"hello server!")
}

func main(){
	godotenv.Load()
	logger.Init()

	exporter:= trace.NewExporter(os.Getenv("EXPORTER_URL"))
	exporter.Start()
	trace.GlobalExporter=exporter


	http.Handle("/hello",observeMiddleware(http.HandlerFunc(helloHandler)))
	http.Handle("/metrics",metrics.Default.Handler())

	// fmt.Println("server is listening on 3000")
	// http.ListenAndServe(":3000",nil)

	server:= &http.Server{Addr: ":3000"}

	//goroutine used so ListenAndServe do not block forever
	go func(){
		logger.L.Info("server started","port",3000)
		server.ListenAndServe()		
	}()

	//main goroutine recieves shutdown signal
	quit:=make(chan os.Signal,1)
	signal.Notify(quit,syscall.SIGINT, syscall.SIGTERM) 
	<-quit

	logger.L.Info("shutting down")
	//shutdown 
	exporter.Shutdown(context.Background())
	server.Shutdown(context.Background())
}