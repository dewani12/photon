package trace

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"github.com/dewani12/photon/pkg/metrics"
	// "github.com/dewani12/observe/logger"
	//cycle 
)

var GlobalExporter *OTLPExporter

type ctxKey struct{}

type responseWriter struct{
	http.ResponseWriter 
	status int
	wroteHeader bool
}

var(
	errorsTotal=metrics.NewCounter(
		"http_errors_total",
		"Total number of HTTP 5xx errors",
		nil,
	)

	requestsTotal=metrics.NewCounter(
		"http_requests_total",
		"Total number of HTTP requests",
		nil,
	)
)

func init(){
	metrics.Default.Register(requestsTotal)
	metrics.Default.Register(errorsTotal)
}

func (rw *responseWriter)WriteHeader(code int){
	if rw.wroteHeader{
		return 
	}
	rw.status=code
	rw.wroteHeader=true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter)Flush(){
	if flusher,ok:=rw.ResponseWriter.(http.Flusher);ok{
		flusher.Flush()
	}
}

func SpanFromContext(ctx context.Context) *Span{
	s,_:=ctx.Value(ctxKey{}).(*Span)
	return s
}

func StartSpan(ctx context.Context,name string)(*Span,context.Context){
	parent,_ := ctx.Value(ctxKey{}).(*Span)

	span:=&Span{
		SpanID: newSpan(),
		Name: name,
		StartTime: time.Now(),
	}

	if parent!=nil {
		span.ParentID=parent.SpanID
		span.TraceID=parent.TraceID
	}else{
		span.TraceID=newTrace()
	}

	return span,context.WithValue(ctx,ctxKey{},span)
}

func Middleware(next http.Handler)http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		traceID,parentID:=ParseTraceparent(r.Header.Get("traceparent"))

		span:=&Span{
			SpanID: newSpan(),
			Name: fmt.Sprintf("%s %s",r.Method,r.URL.Path),
			StartTime: time.Now(),
			Attributes: map[string]string{
				"http.method": r.Method,
				"http.path": r.URL.Path,
			},
		}

		if traceID!=""{
			span.TraceID = traceID
            span.ParentID = parentID
		}else{
			span.TraceID = newTrace()
		}

		w.Header().Set("traceparent",FormatTraceparent(span))

		ctx:=context.WithValue(r.Context(),ctxKey{},span)

		//store trace correlated logger in context
		// l:=logger.WithSpan(ctx)
		// ctx=logger.WithContext(l,ctx)
		// l.Info(
		// 	"request started",
		// 	"method",r.Method,
		// 	"path",r.URL.Path,
		// )

		rw:=&responseWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(rw,r.WithContext(ctx))
		requestsTotal.Inc()
		
		//set attribute
		span.SetAttribute("http.status_code",fmt.Sprintf("%d",rw.status))

		
		if rw.status>=500{
			errorsTotal.Inc()
			span.Status=StatusError
		}else{
			span.Status=StatusOK
		}
			
		span.End()
		// l.Info(
		// 	"request completed",
		// 	"status",rw.status,
        //     "duration",span.Duration().String(),
		// )

		//exporter
		// printSpan(span)
	})
}

//print span or exporter
// func printSpan(s *Span){
// 	fmt.Printf("[SPAN] trace=%s span=%s parent=%s name=%q duration=%s status=%d attrs=%v\n",s.TraceID, s.SpanID, s.ParentID, s.Name, s.Duration(), s.Status, s.Attributes)

// 	if GlobalExporter!=nil{
// 		GlobalExporter.Export(s)
// 	}
// }
func exportSpan(s *Span) {
    fmt.Printf("[SPAN] trace=%s span=%s parent=%s name=%q duration=%s status=%d attrs=%v\n",
        s.TraceID, s.SpanID, s.ParentID, s.Name, s.Duration(), s.Status, s.Attributes)

    if GlobalExporter != nil {
        GlobalExporter.Export(s)
    }
}

