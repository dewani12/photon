package metrics

import (
	"sync/atomic"
)

type Histogram struct {
	name    string
	help    string
	labels  map[string]string
	buckets []float64 //predefined buckets le - upper bound
	counts  []atomic.Uint64
	sum     atomic.Uint64
	total   atomic.Uint64
}
