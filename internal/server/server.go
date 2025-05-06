package server

import (
	"net/http"

	"github.com/openmcp-project/ui-backend/pkg/k8s"
)

func NewMiddleware(theCrateKube k8s.Kube, theDownstreamKube k8s.Kube) *http.ServeMux {
	shared := &shared{
		crateKube:      theCrateKube,
		downstreamKube: theDownstreamKube,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/managed", defaultHandler(shared, managedHandler))
	mux.HandleFunc("/c/", defaultHandler(shared, categoryHandler))
	mux.HandleFunc("/", defaultHandler(shared, mainHandler))

	return mux
}
