package metrics

import (
	"fmt"
	"math"
	"sort"
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

var DefaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05,
	0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
}

func NewHistogram(name, help string, labels map[string]string, buckets []float64) *Histogram {
	if buckets == nil {
		buckets = DefaultBuckets
	}

	sort.Float64s(buckets)

	return &Histogram{
		name:    name,
		help:    help,
		labels:  labels,
		buckets: buckets,
		counts:  make([]atomic.Uint64, len(buckets)),
	}
}

func (h *Histogram) Observe(v float64) {
	for i, le := range h.buckets {
		if v <= le {
			h.counts[i].Add(1)
		}
	}

	for {
		old := h.sum.Load()
		new := math.Float64bits(math.Float64frombits(old) + v)
		//update only if nothing changes after i read
		if h.sum.CompareAndSwap(old, new) {
			break
		}
	}

	h.total.Add(1)
}

func (h *Histogram) Render() string {
	out := fmt.Sprintf("# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)

	labelStr := renderLabels(h.labels)
	prefix := ""
	if labelStr == "" {
		prefix = h.name
	} else {
		prefix = h.name + labelStr[:len(labelStr)-1]
	}

	for i, bound := range h.buckets {
		var leLabel string
		if labelStr == "" {
			leLabel = fmt.Sprintf(`{le="%g"}`, bound)
		} else {
			leLabel = fmt.Sprintf(`,le="%g"}`, bound)
		}
		out += fmt.Sprintf("%s%s %d\n", prefix, leLabel, h.counts[i].Load())
	}

	infLabel := ""
	if labelStr == "" {
		infLabel = `{le="+Inf"}`
	} else {
		infLabel = labelStr[:len(labelStr)-1] + `,le="+Inf"}`
	}
	out += fmt.Sprintf("%s%s %d\n", h.name, infLabel, h.total.Load())
	out += fmt.Sprintf("%s_sum%s %g\n", h.name, labelStr, math.Float64frombits(h.sum.Load()))
	out += fmt.Sprintf("%s_count%s %d\n", h.name, labelStr, h.total.Load())

	return out
}
