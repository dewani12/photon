package logger

import (
	"context"
	"os"
	"log/slog"
	"github.com/dewani12/photon/pkg/trace"
)

type ctxKey struct{}

var L *slog.Logger

func Init() {
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })
    L = slog.New(handler)
    slog.SetDefault(L)
}

//pulls logger from context, if any
func FromContext(ctx context.Context)*slog.Logger{
	if l,ok:=ctx.Value(ctxKey{}).(*slog.Logger);ok{
		return l
	}
	return L
}

//stores the logger in ctx
func WithContext(l *slog.Logger, ctx context.Context)context.Context{
	return context.WithValue(ctx,ctxKey{},l)
}

//attach fields to log of active span in context 
func WithSpan(ctx context.Context)*slog.Logger{
	l:=FromContext(ctx)
	
	s:=trace.SpanFromContext(ctx)
	if s==nil {
		return l
	}

	return l.With(
		"traceID", s.TraceID,
        "spanID",  s.SpanID,
	)
}