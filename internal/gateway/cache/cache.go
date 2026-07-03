// cache-key partitioning by model + system prompt + decoding params (and often tenant), and a conservative similarity threshold to avoid “wrong answer, fast”.

package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type CacheKey struct {
	Model        string
	SystemPrompt string
	Temperature  float64
	TopP         float64
	MaxTokens    int
	TenantID     string
}

func (k CacheKey) Hash() string {
	s := fmt.Sprintf("%s|%s|%.2f|%.2f|%d|%s", k.Model, k.SystemPrompt, k.Temperature, k.TopP, k.MaxTokens, k.TenantID)
	checksum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(checksum[:])
}

//cached entry for query-response pair
type Entry struct {
	Prompt       string
	Embedding    []float64
	Response     []byte
	Model        string
	CreatedAt    time.Time
	HitCount     int
	TokensUsed   int
	PartitionKey string
}

//TODO
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

type MemoryCache struct {
	config Config
	stats  Stats
	// entries []*Entry
	partitions map[string][]*Entry //partition key-> entries
	mu         sync.RWMutex
}

//in-memory implementation
type Cache interface {
	Get(k CacheKey, embedding []float64) Result

	Set(k CacheKey, prompt string, embedding []float64, response []byte, model string, tokens int)

	Stats() Stats

	//removes stale responses due to model updates
	Invalidate(k CacheKey)
}

func DefaultConfig() Config {
	return Config{
		//Threshold: 0.92,
		MaxEntries: 1000,
		TTL:        24 * time.Hour,
	}
}

func NewMemoryCache(cfg Config) *MemoryCache {
	return &MemoryCache{
		config:     cfg,
		partitions: make(map[string][]*Entry),
	}
}
