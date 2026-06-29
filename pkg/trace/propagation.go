package trace

import (
	"fmt"
	"strings"
)

func ParseTraceparent(header string) (traceID,parentSpanID string){
	parts:=strings.Split(header,"-")
	if len(parts) != 4 {
        return "", ""
    }
    if len(parts[1]) != 32 || len(parts[2]) != 16 {
        return "", ""
    }
    return parts[1], parts[2]
}

func FormatTraceparent(s *Span)string{
	return fmt.Sprintf("00-%s-%s-01", s.TraceID, s.SpanID)
}

