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

var (
	btpSOResources = []string{
		"/apis/services.cloud.sap.com/v1/servicebindings",
		"/apis/services.cloud.sap.com/v1/serviceinstances",
		//"/apis/services.cloud.sap.com/v1alpha1/servicebindings",
		//"/apis/services.cloud.sap.com/v1alpha1/serviceinstances",
	}
)

func btpSOHandler(s *shared, req *http.Request, res *response) (*response, *HttpError) {
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
		config, err = openmcp.GetControlPlaneKubeconfig(s.crateKube, data.ProjectName, data.WorkspaceName, data.McpName, data.CrateAuthorization, crateKubeconfig)
		if err != nil {
			slog.Error("failed to get control plane api config", "err", err)
			return nil, NewInternalServerError("failed to get control plane api config")
		}
		if data.McpAuthorization != "" {
			config.SetUserToken(data.McpAuthorization)
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

	res.AddHeader("X-Response-From-Controlplane", "true")

	resBody := make([][]byte, 0)
	headers := data.Headers
	headers["Accept"] = []string{"application/json"}
	for _, resource := range btpSOResources {
		apiReq := k8s.Request{
			Method:  data.Method,
			Path:    resource,
			Headers: headers,
		}

		k8sResp, err := s.downstreamKube.RequestApiServerRaw(apiReq, config)
		if err != nil {
			slog.Error("failed to make request to the api server", "err", err)
			return nil, NewHttpError(http.StatusBadGateway, "failed to make request to the api server")
		}

		defer func(Body io.ReadCloser) {
			errC := Body.Close()
			if errC != nil {
				slog.Error("failed to close api server response body", "err", errC)
			}
		}(k8sResp.Body)

		data, err := io.ReadAll(k8sResp.Body)
		if err != nil {
			slog.Error("failed to read api server response body", "err", err)
			return nil, NewInternalServerError("failed to read api server response body")
		}

		resBody = append(resBody, data)
	}

	var result = append([]byte("["), bytes.Join(resBody, []byte(","))[:]...)
	result = append(result, []byte("]")[:]...)

	if data.JsonPath != "" {
		result, err = ParseJsonPath(result, data.JsonPath)
		if err != nil {
			slog.Error("failed to parse json path", "err", err)
			return nil, NewInternalServerError("failed to parse json path")
		}
	} else if data.JQ != "" {
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
