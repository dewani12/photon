package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dewani12/photon/internal/gateway/cache"
	"github.com/dewani12/photon/pkg/logger"
	"github.com/dewani12/photon/pkg/trace"
)

//open AI compatible structures

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	//for cache key
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	MaxTokens   int     `json:"max_tokens"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// type Gateway struct{
// 	upstreamURL string
// 	apiKey 		string
// 	client 		*http.Client
// }

// SSE Enable
type ChatChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage"` //in last chunk
}

type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"` //in last chunk
}

type Delta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func buildCacheContext(req ChatRequest) (cache.CacheKey, string) {
	var systemPrompt string
	parts := make([]string, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" && systemPrompt == "" {
			systemPrompt = msg.Content
		}
		parts = append(parts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	return cache.CacheKey{
		Model:        req.Model,
		SystemPrompt: systemPrompt,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
		MaxTokens:    req.MaxTokens,
	}, strings.Join(parts, "\n")
}

func (g *Gateway) ChatHandler(w http.ResponseWriter, r *http.Request) {
	span, ctx := trace.StartSpan(r.Context(), "gateway.chat")
	defer span.End()

	l := logger.WithSpan(ctx)
	l.Info("chat request recieved")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		l.Error("failed to read body", "error", err)
		span.Status = trace.StatusError
		http.Error(w, "failed to read body", 400)
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		l.Error("invalid JSON", "error", err)
		span.Status = trace.StatusError
		http.Error(w, "invalid JSON", 400)
		return
	}

	span.SetAttribute("llm.model", req.Model)
	span.SetAttribute("llm.message_count", fmt.Sprintf("%d", len(req.Messages)))
	span.SetAttribute("llm.stream", fmt.Sprintf("%v", req.Stream))

	//TODO: estimate input token before llm call

	cacheKey, prompt := buildCacheContext(req)
	var cacheEmbedding []float64
	if !req.Stream {
		var err error
		cacheEmbedding, err = g.embedder.Embed(prompt)
		if err != nil {
			l.Warn("cache embedding failed", "error", err)
		} else {
			result := g.cache.Get(cacheKey, cacheEmbedding)
			if result.Hit && result.Entry != nil {
				l.Info("semantic cache hit", "score", result.Score)
				cache.RecordHit(result.Entry.TokensUsed, result.Score)
				span.SetAttribute("cache.hit", "true")
				span.SetAttribute("cache.score", fmt.Sprintf("%.3f", result.Score))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(result.Entry.Response)
				return
			}
			cache.RecordMiss(result.Score)
			span.SetAttribute("cache.hit", "false")
		}
	}

	l.Info("forwarding to upstream",
		"model", req.Model,
		"messages", len(req.Messages),
	)

	//measure TTFT - latency metrics
	start := time.Now()

	if req.Stream {
		g.handleStream(w, r, span, l, body, start)
		return
	}

	res, body, err := g.forward(ctx, body)

	if err != nil {
		l.Error("upstream call failed", "error", err)
		span.Status = trace.StatusError
		llmErrorsTotal.Inc()
		http.Error(w, "upstream error", 502)
		return
	}

	ttft := time.Since(start)

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		l.Error("failed to parse upstream response", "error", err)
		span.Status = trace.StatusError
		http.Error(w, "upstream parse error", 502)
		return
	}

	//set attributes
	span.SetAttribute("llm.ttft_ms", fmt.Sprintf("%d", ttft.Milliseconds()))

	l.Info("upstream responsed",
		"model", resp.Model,
		"choices", len(resp.Choices),
		"usage_prompt_tokens", resp.Usage.PromptTokens,
		"usage_completion_tokens", resp.Usage.CompletionTokens,
		"usage_total_tokens", resp.Usage.TotalTokens,
		"response_id", resp.ID,
	)
	if len(resp.Choices) > 0 {
		l.Info("upstream response content",
			"content", resp.Choices[0].Message.Content,
			"role", resp.Choices[0].Message.Role,
			"finish_reason", resp.Choices[0].FinishReason,
		)
	}

	//update metrics
	llmRequestsTotal.Inc()

	if !req.Stream && len(cacheEmbedding) > 0 {
		tokens := resp.Usage.TotalTokens
		if tokens == 0 {
			tokens = req.MaxTokens
		}
		g.cache.Set(cacheKey, prompt, cacheEmbedding, body, resp.Model, tokens)
	}

	l.Info("upstream responded")

	span.Status = trace.StatusOK

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	w.Write(body)
}

// returns raw response
func (g *Gateway) forward(ctx context.Context, body []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		g.config.UpstreamURL,
		bytes.NewReader(body),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.config.APIKey)

	res, err := g.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("upstream call: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	return res, respBody, nil
}

func (g *Gateway) forwardStream(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		g.config.UpstreamURL,
		bytes.NewReader(body),
	)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	// g.client.Do(req)
	client := &http.Client{}

	res, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("upstream call: %w", err)
	}

	return res, nil
}

func (g *Gateway) handleStream(w http.ResponseWriter, r *http.Request, span *trace.Span, l *slog.Logger, body []byte, start time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		l.Error("client does not support streaming")
		http.Error(w, "streaming not supported", 500)
		return
	}

	//SSE header
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")

	res, err := g.forwardStream(r.Context(), body)
	if err != nil {
		l.Info("upstream stream failed", "error", err)
		span.Status = trace.StatusError
		return
	}

	defer res.Body.Close()

	scanner := bufio.NewScanner(res.Body)

	var firstChunk = true
	var ttft time.Duration

	for scanner.Scan() {
		line := scanner.Text()

		//separators
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			flusher.Flush()
			break
		}

		var chunk ChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			l.Error("failed to parse chunk", "error", err, "data", data)
			continue
		}

		if firstChunk {
			ttft = time.Since(start)
			span.SetAttribute("llm.ttft_ms", fmt.Sprintf("%d", ttft.Milliseconds()))
			firstChunk = false
			l.Info("first token received", "ttft_ms", ttft.Milliseconds())
		}

		if chunk.Usage != nil {

		}

		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		l.Error("stream read error", "error", err)
		span.Status = trace.StatusError
		return
	}

	span.Status = trace.StatusOK

	l.Info("stream completed")
}
