package gateway

import (
	"github.com/dewani12/photon/pkg/metrics"
)

var (
	llmRequestsTotal = metrics.NewCounter(
		"llm_requests_total",
		"Total LLM requests processed by gateway",
		nil,
	)

	llmErrorsTotal = metrics.NewCounter(
		"llm_errors_total",
		"Total LLM requests errors",
		nil,
	)
)

func Init() {
	metrics.Default.Register(llmRequestsTotal)
	metrics.Default.Register(llmErrorsTotal)
}
