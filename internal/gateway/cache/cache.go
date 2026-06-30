// cache-key partitioning by model + system prompt + decoding params (and often tenant), and a conservative similarity threshold to avoid “wrong answer, fast”.

package cache

import (
	"sync"
	"time"
)

//cached entry for query-response pair
type Entry struct {
	Prompt    string
	Embedding []float64
	Response  []byte
	Model     string
	CreatedAt time.Time
}

//params hash
//tenant/user scope

//result of cache lookup
type Result struct {
	Hit   bool
	Entry *Entry
	Score float64
}

//cache performance no.
type Stats struct {
	TotalEntries int
	Hits         int64
	Misses       int64
	TokensSaved  int64
}

//cache configuration
type Config struct {
	Threshold  float64
	MaxEntries int
	TTL        time.Duration
}

//in-memory implementation
type MemoryCache struct {
	config  Config
	stats   Stats
	entries *[]Entry
	mu      sync.RWMutex
}
