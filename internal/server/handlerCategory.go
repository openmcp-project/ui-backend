package server

import (
	"bytes"
	"context"
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
		if data.McpVersion == "v2" {
			config, err = openmcp.GetControlPlaneV2Kubeconfig(s.crateKube, data.ProjectName, data.WorkspaceName, data.McpName, data.McpIdp, data.CrateAuthorizationToken, crateKubeconfig)
		} else {
			config, err = openmcp.GetControlPlaneKubeconfig(s.crateKube, data.ProjectName, data.WorkspaceName, data.McpName, data.CrateAuthorizationToken, crateKubeconfig)
		}
		if err != nil {
			slog.Error("failed to get control plane api config", "err", err)
			return nil, NewInternalServerError("failed to get control plane api config")
		}
		if data.McpAuthorizationToken == "" {
			slog.Error("MCP authorization token not provided")
			return nil, NewBadRequestError("MCP authorization token not provided")
		}
		config.SetUserToken(data.McpAuthorizationToken)
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
	// fetchResource performs a single api server request and returns the body,
	// closing the response body via defer so it is released every iteration
	// (a bare defer in the loop below would accumulate until function return).
	fetchResource := func(apiReq k8s.Request) ([]byte, *HttpError) {
		k8sResp, err := s.downstreamKube.RequestApiServerRaw(apiReq, config)
		if err != nil {
			slog.Error("failed to get managed resources", "err", err)
			return nil, NewInternalServerError("failed to get managed resources")
		}
		defer k8sResp.Body.Close()

		body, err := io.ReadAll(k8sResp.Body)
		if err != nil {
			slog.Error("failed to read data from response", "err", err)
			return nil, NewInternalServerError("failed to read data from response")
		}
		return body, nil
	}

	for _, category := range categories {
		for _, version := range category.Versions {
			for _, resource := range version.Resources {
				apiReq := k8s.Request{
					Method:  "GET",
					Path:    "/apis/" + category.Name + "/" + version.Version + "/" + resource.Resource,
					Headers: data.Headers,
				}

				body, httpErr := fetchResource(apiReq)
				if httpErr != nil {
					return nil, httpErr
				}

				resultData = append(resultData, body)
			}
		}
	}

	var result []byte = append([]byte("["), bytes.Join(resultData, []byte(","))[:]...)
	result = append(result, []byte("]")[:]...)

	if data.JQ != "" {
		if len(data.JQ) > s.jqConfig.MaxExpressionLength {
			return nil, NewBadRequestError("jq expression exceeds maximum allowed length")
		}

		ctx, cancel := context.WithTimeout(req.Context(), s.jqConfig.ExecutionTimeout)
		defer cancel()

		resultString, err := ParseJQ(ctx, result, data.JQ, s.jqConfig.MaxResults)
		if err != nil {
			slog.Error("jq execution failed", "err", err)
			return nil, NewInternalServerError("failed to process jq expression")
		}

		result = []byte(resultString)
	}

	res.body = result

	return res, nil
}
