package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/openmcp-project/ui-backend/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type shared struct {
	crateKube      k8s.Kube
	downstreamKube k8s.Kube
}

type handler func(shared *shared, req *http.Request, res *response) (*response, *HttpError)

type response struct {
	body        []byte
	contentType string
	statusCode  int
	headers     map[string]string
}

func (r *response) AddHeader(key, value string) {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
}

func defaultHandler(shared *shared, handlerFunc handler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if (*req).Method == "OPTIONS" {
			handleOptions(w)
			return
		}

		res := &response{}
		res, err := handlerFunc(shared, req, res)
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if err != nil {
			slog.Error("request processing failed", "err", err)

			status := err.ToAPIStatus()
			var encoder = unstructured.NewJSONFallbackEncoder(unstructured.UnstructuredJSONScheme)
			output, errEnc := runtime.Encode(encoder, status)
			if errEnc != nil {
				output = []byte(fmt.Sprintf("%s: %s", status.Reason, status.Message))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(int(status.Code))
			if _, errWrite := w.Write(output); errWrite != nil {
				utilruntime.HandleError(fmt.Errorf("proxy was unable to write a fallback JSON response: %v", errWrite))
			}
			return
		}

		if res.contentType != "" {
			w.Header().Set("Content-Type", res.contentType)
		}
		if res.headers != nil {
			for k, v := range res.headers {
				w.Header().Set(k, v)
			}
		}
		if res.statusCode > 0 {
			w.WriteHeader(res.statusCode)
		}
		if _, errWrite := w.Write(res.body); errWrite != nil {
			slog.Error("can't write response", "err", err)
			utilruntime.HandleError(fmt.Errorf("was unable to write a response: %v", errWrite))
		}
	}
}

func handleOptions(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.WriteHeader(http.StatusOK)
}
