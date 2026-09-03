package server

import (
	"net/http"

	"github.com/openmcp-project/ui-backend/pkg/k8s"
)

func NewMiddleware(theCrateKube k8s.Kube, theDownstreamKube k8s.Kube, jqConfig JQConfig) *http.ServeMux {
	shared := &shared{
		crateKube:      theCrateKube,
		downstreamKube: theDownstreamKube,
		jqConfig:       jqConfig,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/managed", defaultHandler(shared, managedHandler))
	mux.HandleFunc("/c/", defaultHandler(shared, categoryHandler))
	mux.HandleFunc("/", defaultHandler(shared, mainHandler))

	return mux
}
