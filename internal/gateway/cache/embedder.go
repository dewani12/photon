package cache

import (
	"net/http"
	"os"
	"time"

	"github.com/dewani12/photon/pkg/config"
)

type Embedder struct{
	apiKey   	string
	endpoint 	string
	model    	string
	client   	*http.Client
	dimensions 	int
}

type embedRequest struct{
	Model string `json:"mode"`
	Input string `json:"input"`
	//encoding_format
}

type Usage struct{
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type Data struct{
	Object 		string		`json:"object"`
	Index 		int			`json:"index"`
	Embedding 	[]float64   `json:"embedding"`
}

type embedResponse struct{
	Object 	string	`json:"object"`
	Data 	[]Data 	`json:"data"`
	Model 	string	`json:"model"`
	Usage 	Usage	`json:"usage"`
}

func NewEmbedder()*Embedder{
	return &Embedder{
		apiKey: os.Getenv("EMBEDDER_API_KEY"),
		endpoint: config.GetEnv("EMBEDDER_URL","https://api.openai.com/v1/embeddings"),
		model: config.GetEnv("EMBEDDER_MODEL","text-embedding-3-small"),
		client: &http.Client{
			Timeout: 10*time.Second,
		},
	}
}

// func (e *Embedder)Embed(text string)([]float64,error){

// }
