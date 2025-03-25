package server

import (
	"log/slog"
	"net/http"

	"github.com/openmcp-project/ui-backend/internal/utils"
	"gopkg.in/yaml.v3"
)

func wellKnownKubeconfigHandler(_ *shared, _ *http.Request, res *response) (*response, *HttpError) {
	crateKubeconfig, ok := utils.GetCrateKubeconfig()
	if !ok {
		slog.Error("failed to get crate kubeconfig")
		return nil, NewInternalServerError("failed to get crate kubeconfig")
	}
	content, err := yaml.Marshal(crateKubeconfig)
	if err != nil {
		slog.Error("failed to marshal kubeconfig", "err", err)
		return nil, NewInternalServerError("failed to marshal kubeconfig")
	}
	res.body = content
	res.contentType = "application/yaml"

	return res, nil
}
