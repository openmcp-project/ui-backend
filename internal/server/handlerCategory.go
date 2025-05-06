package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/openmcp-project/ui-backend/internal/utils"
	"github.com/openmcp-project/ui-backend/pkg/k8s"
	"github.com/openmcp-project/ui-backend/pkg/openmcp"
)

func managedHandler(s *shared, req *http.Request, res *response) (*response, *HttpError) {
	req.Header.Set("X-category", "managed")
	return _categoryHandler(s, req, res)
}

func categoryHandler(s *shared, req *http.Request, res *response) (*response, *HttpError) {
	path := req.URL.Path
	// cut off the "/c/" prefix
	if len(path) < 3 {
		return nil, NewBadRequestError("invalid request")
	}
	if len(path) > 3 {
		req.Header.Set("X-category", path[3:])
	}

	return _categoryHandler(s, req, res)
}

// This handler creates an endpoint for the client to get all crossplane managed resources.
func _categoryHandler(s *shared, req *http.Request, res *response) (*response, *HttpError) {
	data, err := extractRequestData(req)
	if err != nil {
		return nil, NewBadRequestError("invalid request")
	}

	DeleteMultiple(data.Headers, prohibitedRequestHeaders)

	crateKubeconfig, ok := utils.GetCrateKubeconfig()
	if !ok {
		slog.Error("failed to get crate kubeconfig")
		return nil, NewInternalServerError("failed to get crate kubeconfig")
	}

	var config k8s.KubeConfig
	if data.ProjectName != "" && data.WorkspaceName != "" && data.McpName != "" {
		config, err = openmcp.GetControlPlaneKubeconfig(s.crateKube, data.ProjectName, data.WorkspaceName, data.McpName, data.Authorization, crateKubeconfig)
		if err != nil {
			slog.Error("failed to get control plane api config", "err", err)
			return nil, NewInternalServerError("failed to get control plane api config")
		}
		if data.Authorization != "" {
			config.SetUserToken(data.Authorization)
		}
	} else {
		slog.Error("either use %s: true or provide %s, %s and %s headers", useCrateClusterHeader, projectNameHeader, workspaceNameHeader, mcpName)
		return nil, NewBadRequestError(
			"either use %s: true or provide %s, %s and %s headers",
			useCrateClusterHeader,
			projectNameHeader,
			workspaceNameHeader,
			mcpName,
		)
	}

	if data.Category == "" {
		return nil, NewBadRequestError("category not provided")
	}

	res.AddHeader("X-Response-From-Controlplane", "true")

	categories, err := s.downstreamKube.RequestApiGroupsByCategory(config, data.Category)
	if err != nil {
		slog.Error("failed to get managed resources", "err", err)
		return nil, NewInternalServerError("failed to get managed resources")
	}

	resultData := make([][]byte, 0)
	for _, category := range categories {
		for _, version := range category.Versions {
			for _, resource := range version.Resources {
				apiReq := k8s.Request{
					Method:  "GET",
					Path:    "/apis/" + category.Name + "/" + version.Version + "/" + resource.Resource,
					Headers: data.Headers,
				}

				k8sResp, err := s.downstreamKube.RequestApiServerRaw(apiReq, config)
				if err != nil {
					slog.Error("failed to get managed resources", "err", err)
					return nil, NewInternalServerError("failed to get managed resources")
				}

				data, err := io.ReadAll(k8sResp.Body)
				if err != nil {
					slog.Error("failed to read data from response", "err", err)
					return nil, NewInternalServerError("failed to read data from response")
				}

				resultData = append(resultData, data)
			}
		}
	}

	var result []byte = append([]byte("["), bytes.Join(resultData, []byte(","))[:]...)
	result = append(result, []byte("]")[:]...)

	if data.JQ != "" {
		resultString, err := ParseJQ(result, data.JQ)
		if err != nil {
			slog.Error("failed to parse jq", "err", err)
			return nil, NewInternalServerError("failed to parse jq")
		}

		result = []byte(resultString)
	}

	res.body = result

	return res, nil
}
