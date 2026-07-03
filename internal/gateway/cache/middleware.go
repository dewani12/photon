package cache

import (
	"github.com/dewani12/photon/pkg/metrics"
)

var (
	cacheHits = metrics.NewCounter(
		"llm_cache_hits_total",
		"Total semantic cache hits",
		nil,
	)

	cacheMisses = metrics.NewCounter(
		"llm_cache_misses_total",
		"Total semantic cache misses",
		nil,
	)

	cacheTokensSaved = metrics.NewCounter(
		"llm_cache_tokens_saved_total",
		"Total tokens saved by semantic cache",
		nil,
	)
)

func Init() {
	metrics.Default.Register(cacheHits)
	metrics.Default.Register(cacheMisses)
	metrics.Default.Register(cacheTokensSaved)
}

func RecordHit(tokensSaved int, score float64) {
	cacheHits.Inc()
	cacheTokensSaved.Add(uint64(tokensSaved))
}

func RecordMiss(score float64) {
	cacheMisses.Inc()
}
