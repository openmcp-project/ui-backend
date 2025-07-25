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
	useCrateClusterHeader                 = "X-use-crate"
	authorizationHeader                   = "Authorization"
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
	authorizationHeader,
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
	ContextName                     string
	UseCrateCluster                 bool
	CrateAuthorizationToken         string
	McpAuthorizationToken           string
	Headers                         map[string][]string
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
		config.SetUserToken(data.CrateAuthorizationToken)
	} else if data.ProjectName != "" && data.WorkspaceName != "" && data.McpName != "" {
		config, err = openmcp.GetControlPlaneKubeconfig(s.crateKube, data.ProjectName, data.WorkspaceName, data.McpName, data.CrateAuthorizationToken, crateKubeconfig)
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

	if (data.JQ == "") || k8sResp.StatusCode >= 400 {
		err = CopyResponse(res, k8sResp, nil, nil)
		if err != nil {
			return nil, NewInternalServerError("failed to copy response: %v", err)
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
	if r.Header.Get(authorizationHeader) == "" {
		return ExtractedRequestData{}, fmt.Errorf("%s header is required", authorizationHeader)
	}

	crateToken, mcpToken, err := parseAuthorizationHeaderWithDoubleTokens(r.Header.Get(authorizationHeader))
	if err != nil {
		return ExtractedRequestData{}, fmt.Errorf("invalid %s header: %w", authorizationHeader, err)
	}

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
		McpName:                         r.Header.Get(mcpName),
		CrateAuthorizationToken:         crateToken,
		McpAuthorizationToken:           mcpToken,
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
