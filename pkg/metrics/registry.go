package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type renderer interface {
	Render() string
}

type Registry struct {
	mu      sync.RWMutex
	metrics []renderer
}

var Default = &Registry{}

func (r *Registry) Register(m renderer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, m)
}

func (r *Registry) Render() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	for _, m := range r.metrics {
		sb.WriteString(m.Render())
		sb.WriteString("\n")
	}
	return sb.String()
}

func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.0")
		fmt.Fprint(w, r.Render())
	}
}

//convert map to prometheus label format
func renderLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
