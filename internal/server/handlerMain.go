package server

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/openmcp-project/ui-backend/internal/utils"
	"github.com/openmcp-project/ui-backend/pkg/k8s"
	"github.com/openmcp-project/ui-backend/pkg/openmcp"
)

const (
	clientCertificateDataHeader           = "X-Client-Certificate-Data"
	clientKeyDataHeader                   = "X-Client-Key-Data"
	clusterCertificateAuthorityDataHeader = "X-Cluster-Certificate-Authority-Data"
	projectNameHeader                     = "X-project"
	workspaceNameHeader                   = "X-workspace"
	mcpName                               = "X-mcp"
	mcpAuthHeader                         = "X-mcp-authorization"
	contextHeader                         = "X-context"
	useCrateClusterHeader                 = "X-use-crate"
	authorizationHeader                   = "Authorization"
	jsonPathHeader                        = "X-jsonpath"
	jqHeader                              = "X-jq"
	categoryHeader                        = "X-category"
)

var prohibitedRequestHeaders = []string{
	clientCertificateDataHeader,
	clientKeyDataHeader,
	clusterCertificateAuthorityDataHeader,
	projectNameHeader,
	workspaceNameHeader,
	mcpName,
	mcpAuthHeader,
	contextHeader,
	authorizationHeader,
	jsonPathHeader,
	"User-Agent",
	"Host",
}

type ExtractedRequestData struct {
	Path                            string
	Query                           url.Values
	Body                            io.Reader
	Method                          string
	ClientCertificateData           string
	ClientKeyData                   string
	ClusterCertificateAuthorityData string
	ProjectName                     string
	WorkspaceName                   string
	McpName                         string
	McpAuthorization                string
	ContextName                     string
	UseCrateCluster                 bool
	CrateAuthorization              string
	Headers                         map[string][]string
	JsonPath                        string
	JQ                              string
	Category                        string
}

var prohibitedResponseHeaders = []string{"Content-Type", "Content-Length"}

func mainHandler(s *shared, req *http.Request, res *response) (*response, *HttpError) {
	data, err := extractRequestData(req)
	if err != nil {
		return nil, NewBadRequestError("invalid request")
	}

	DeleteMultiple(data.Headers, prohibitedRequestHeaders)

	apiReq := k8s.Request{
		Method:  data.Method,
		Path:    data.Path,
		Query:   data.Query,
		Body:    data.Body,
		Headers: data.Headers,
	}

	crateKubeconfig, ok := utils.GetCrateKubeconfig()
	if !ok {
		slog.Error("failed to get crate kubeconfig")
		return nil, NewInternalServerError("failed to get crate kubeconfig")
	}

	var config k8s.KubeConfig
	if data.UseCrateCluster {
		config = crateKubeconfig
		config.SetUserToken(data.CrateAuthorization)
	} else if data.ProjectName != "" && data.WorkspaceName != "" && data.McpName != "" {
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

	if (data.JsonPath == "" && data.JQ == "") || k8sResp.StatusCode >= 400 {
		err = CopyResponse(res, k8sResp, nil, nil)
		if err != nil {
			return nil, NewInternalServerError("failed to copy response: %v", err)
		}
	} else if data.JsonPath != "" {
		err := res.buildJsonPathResponse(k8sResp, err, data)
		if err != nil {
			return nil, NewInternalServerError("failed to build jsonpath response: %v", err)
		}
	} else {
		err := res.buildJqResponse(k8sResp, data)
		if err != nil {
			return nil, NewInternalServerError("failed to build jq response: %v", err)
		}
	}

	return res, nil
}

func extractRequestData(r *http.Request) (ExtractedRequestData, error) {
	rd := ExtractedRequestData{
		Path:                            r.URL.Path,
		Query:                           r.URL.Query(),
		Body:                            r.Body,
		Method:                          r.Method,
		ClientCertificateData:           r.Header.Get(clientCertificateDataHeader),
		ClientKeyData:                   r.Header.Get(clientKeyDataHeader),
		ClusterCertificateAuthorityData: r.Header.Get(clusterCertificateAuthorityDataHeader),
		ProjectName:                     r.Header.Get(projectNameHeader),
		WorkspaceName:                   r.Header.Get(workspaceNameHeader),
		ContextName:                     r.Header.Get(contextHeader),
		McpAuthorization:                r.Header.Get(mcpAuthHeader),
		McpName:                         r.Header.Get(mcpName),
		CrateAuthorization:              r.Header.Get(authorizationHeader),
		JsonPath:                        r.Header.Get(jsonPathHeader),
		JQ:                              r.Header.Get(jqHeader),
		Category:                        r.Header.Get(categoryHeader),
	}

	rd.Headers = r.Header

	if cc := r.Header.Get(useCrateClusterHeader); cc != "" {
		useCrateCluster, err := strconv.ParseBool(r.Header.Get(useCrateClusterHeader))
		if err != nil {
			return ExtractedRequestData{}, fmt.Errorf("%s has to be a boolean value", useCrateClusterHeader)
		}

		rd.UseCrateCluster = useCrateCluster
	}

	if rd.CrateAuthorization == "" {
		return ExtractedRequestData{}, fmt.Errorf("%s header is required", authorizationHeader)
	}

	return rd, nil
}

func (r *response) buildJqResponse(k8sResp *http.Response, data ExtractedRequestData) error {
	body, err := io.ReadAll(k8sResp.Body)
	if err != nil {
		return errors.Join(errors.New("failed to read api server response"), err)
	}

	parsedJson, err := ParseJQ(body, data.JQ)
	if err != nil {
		return errors.Join(errors.New("failed to parse response with jq"), err)
	}

	err = CopyResponse(r, k8sResp, []byte(parsedJson), prohibitedResponseHeaders)

	r.contentType = "application/json"
	return err
}

func (r *response) buildJsonPathResponse(k8sResp *http.Response, err error, data ExtractedRequestData) error {
	body, errR := io.ReadAll(k8sResp.Body)
	if errR != nil {
		return errors.Join(errors.New("failed to read api server response"), err)
	}

	parsedJson, err := ParseJsonPath(body, data.JsonPath)
	if err != nil {
		return errors.Join(errors.New("failed to parse response with jsonpath"), err)
	}

	err = CopyResponse(r, k8sResp, parsedJson, prohibitedResponseHeaders)
	r.contentType = "application/json"
	return err
}
