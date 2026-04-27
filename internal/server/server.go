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

	mux.HandleFunc("/managed", defaultHandler(shared, managedHandler))
	mux.HandleFunc("/c/", defaultHandler(shared, categoryHandler))
	mux.HandleFunc("/", defaultHandler(shared, mainHandler))

	return mux
}
