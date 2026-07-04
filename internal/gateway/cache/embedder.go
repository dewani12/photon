package cache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dewani12/photon/pkg/config"
)

type Embedder struct {
	apiKey     string
	endpoint   string
	model      string
	client     *http.Client
	dimensions int
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	//encoding_format
}

type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type Data struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type embedResponse struct {
	Object string `json:"object"`
	Data   []Data `json:"data"`
	Model  string `json:"model"`
	Usage  Usage  `json:"usage"`
}

func NewEmbedder() *Embedder {
	return &Embedder{
		apiKey:   os.Getenv("EMBEDDER_API_KEY"),
		endpoint: config.GetEnv("EMBEDDER_URL", "https://api.openai.com/v1/embeddings"),
		model:    config.GetEnv("EMBEDDER_MODEL", "text-embedding-3-small"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// prompt to vector conversion
func (e *Embedder) Embed(text string) ([]float64, error) {
	payload, err := json.Marshal(embedRequest{
		Model: e.model,
		Input: text,
	})

	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequest("POST", e.endpoint, bytes.NewReader(payload))

	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	res, err := e.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("embed call: %w", err)
	}

	defer req.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("embed API error %d: %s", res.StatusCode, string(body))
	}

	var result embedResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return result.Data[0].Embedding, nil
}
