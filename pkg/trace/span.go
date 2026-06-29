package trace

import(
	"time"
	"crypto/rand"
	"encoding/hex"
)

type SpanStatus int

type Event struct{
	Name string
	TimeStamp time.Time
	Attrs map[string]string 
}

const (
	StatusUnset SpanStatus = iota
	StatusOK
	StatusError
)

type Span struct{
	TraceID string
	SpanID string
	ParentID string 
	Name string
	StartTime time.Time
	EndTime time.Time
	Status SpanStatus
	Attributes map[string]string
	Events []Event
}

func newID(bytes int)string{
	b:=make([]byte,bytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func newTrace()string {
	return newID(16)
}

func newSpan()string {
	return newID(8)
}

//methods of Span

func (s *Span)Duration()time.Duration{
	return s.EndTime.Sub(s.StartTime)
}

func (s *Span)SetAttribute(k,v string){
	if s.Attributes == nil {
        s.Attributes = make(map[string]string)
    }
    s.Attributes[k] = v
}

func (s *Span) End() {
    s.EndTime = time.Now()
    exportSpan(s)
}


