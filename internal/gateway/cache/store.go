package cache

import (
	"math"
	"sync/atomic"
	"time"
)

// TODO:implement ANN - HNSW
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (c *MemoryCache) Get(k CacheKey, embedding []float64) Result {
	key := k.Hash()

	c.mu.RLock()
	defer c.mu.RUnlock()

	entries, exists := c.partitions[key]
	if !exists {
		atomic.AddInt64(&c.stats.Misses, 1)
		return Result{
			Hit: false,
		}
	}

	var best *Entry
	var bestScore float64
	now := time.Now()

	for _, entry := range entries {
		//skip expired entries
		if now.Sub(entry.CreatedAt) > c.config.TTL {
			continue
		}

		//calculate similarity score
		score := CosineSimilarity(entry.Embedding, embedding)

		if score > bestScore {
			best = entry
			bestScore = score
		}
	}

	if best != nil && bestScore >= c.config.Threshold {
		atomic.AddInt64(&c.stats.Hits, 1)
		atomic.AddInt64(&c.stats.TokensSaved, int64(best.TokensUsed))
		best.HitCount++ //TODO: atomic opr need?
		return Result{Hit: true, Entry: best, Score: bestScore}
	}

	atomic.AddInt64(&c.stats.Misses, 1)
	return Result{
		Hit:   false,
		Score: bestScore,
	}
}

func (c *MemoryCache) Set(k CacheKey, prompt string, embedding []float64, response []byte, model string, tokens int) {
	key := k.Hash()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &Entry{
		Prompt:       prompt,
		Embedding:    embedding,
		Response:     response,
		Model:        model,
		CreatedAt:    time.Now(),
		TokensUsed:   tokens,
		PartitionKey: key,
	}
	//if full, evict entry

	entries := c.partitions[key]
	c.partitions[key] = append(entries, entry)
}

func (c *MemoryCache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, v := range c.partitions {
		total += len(v)
	}

	return Stats{
		TotalEntries: total,
		Hits:         c.stats.Hits,
		Misses:       c.stats.Misses,
		TokensSaved:  c.stats.TokensSaved,
	}
}

func (c *MemoryCache) Invalidate(k CacheKey) {
	key := k.Hash()
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.partitions, key)
}
