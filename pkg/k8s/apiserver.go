package k8s

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/patrickmn/go-cache"
	"k8s.io/api/apidiscovery/v2beta1"
)

func RequestApiServer(kube Kube, request Request, config KubeConfig, result interface{}) error {
	if request.Headers == nil {
		request.Headers = make(map[string][]string)
	}
	request.Headers["Content-Type"] = []string{"application/json"}

	res, err := kube.RequestApiServerRaw(request, config)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode json response: %v", err)
	}

	return nil
}

type Request struct {
	Method  string
	Path    string
	Query   map[string][]string
	Headers map[string][]string
	Body    io.Reader
}

type Kube interface {
	RequestApiServerRaw(request Request, config KubeConfig) (*http.Response, error)
	RequestApiGroupsByCategory(config KubeConfig, category string) ([]v2beta1.APIGroupDiscovery, error)
}

var _ Kube = HttpKube{}

type HttpKube struct{}

// RequestApiServerRaw It is expected that the config of type Kubeconfig is valid, meaning the arrays .clusters and .users are not empty
func (HttpKube) RequestApiServerRaw(request Request, config KubeConfig) (*http.Response, error) {
	tlsConfig := tls.Config{}
	if len(config.Clusters) == 0 || len(config.Users) == 0 {
		return nil, fmt.Errorf("invalid kubeconfig: empty clusters or users")
	}
	cluster := config.Clusters[0]
	user := config.Users[0]

	if cluster.Cluster.CertificateAuthorityData != "" {
		caCertBytes, err := base64.StdEncoding.DecodeString(cluster.Cluster.CertificateAuthorityData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode CA certificate data: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCertBytes) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	if user.User.ClientCertificateData != "" {
		clientCertBytes, err := base64.StdEncoding.DecodeString(user.User.ClientCertificateData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode client certificate data: %v", err)
		}

		clientKeyBytes, err := base64.StdEncoding.DecodeString(user.User.ClientKeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode client key data: %v", err)
		}

		clientCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to load client key pair: %v", err)
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	requestUrlStr, err := url.JoinPath(cluster.Cluster.Server, request.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to join url path: %v", err)
	}

	requestUrl, err := url.Parse(requestUrlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %v", err)
	}

	for k, v := range request.Query {
		for _, vv := range v {
			requestUrl.Query().Add(k, vv)
		}
	}

	req, err := http.NewRequest(request.Method, requestUrl.String(), request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	if request.Headers != nil {
		for k, v := range request.Headers {
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
	}

	if user.User.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.User.Token))
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}

	slog.Debug("requesting api server", "method", request.Method, "host", config.Clusters[0].Cluster.Server, "path", request.Path)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request api server: %v", err)
	}

	return res, nil
}

func (h HttpKube) RequestApiGroupsByCategory(config KubeConfig, category string) ([]v2beta1.APIGroupDiscovery, error) {
	acceptHeader := "application/json;g=apidiscovery.k8s.io;v=v2;as=APIGroupDiscoveryList,application/json;g=apidiscovery.k8s.io;v=v2beta1;as=APIGroupDiscoveryList,application/json"
	req := Request{
		Method: "GET",
		Path:   "/api",
		Headers: map[string][]string{
			"Accept": {acceptHeader},
		},
	}

	var apiGroups v2beta1.APIGroupDiscoveryList
	if err := RequestApiServer(h, req, config, &apiGroups); err != nil {
		return nil, err
	}

	req = Request{
		Method: "GET",
		Path:   "/apis",
		Headers: map[string][]string{
			"Accept": {acceptHeader},
		},
	}

	var apisGroups v2beta1.APIGroupDiscoveryList
	if err := RequestApiServer(h, req, config, &apiGroups); err != nil {
		return nil, err
	}

	apiGroupItems := append(apiGroups.Items, apisGroups.Items...)

	var filteredItems []v2beta1.APIGroupDiscovery
	for _, item := range apiGroupItems {
		filteredVersions := []v2beta1.APIVersionDiscovery{}
		for _, version := range item.Versions {
			filteredVersion := version
			filteredResources := []v2beta1.APIResourceDiscovery{}
			for _, resource := range version.Resources {
				if resource.Categories != nil {
					for _, cat := range resource.Categories {
						if cat == category {
							filteredResources = append(filteredResources, resource)
						}
					}
				}
			}

			if len(filteredResources) > 0 {
				filteredVersion.Resources = filteredResources
				filteredVersions = append(filteredVersions, filteredVersion)
			}
		}

		if len(filteredVersions) > 0 {
			item.Versions = filteredVersions
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

var _ Kube = cachingKube{}

type cachingKube struct {
	downstream Kube
	cache      *cache.Cache
}

func NewCachingKube(downstream Kube, defaultExpiration, cleanupInterval time.Duration) Kube {
	kube := cachingKube{
		downstream: downstream,
		cache:      cache.New(defaultExpiration, cleanupInterval),
	}
	return &kube
}

func (c cachingKube) RequestApiServerRaw(request Request, config KubeConfig) (*http.Response, error) {
	// this key is a unique identifier for the request - however, it is not guaranteed to be unique
	key := fmt.Sprintf("%s %s %s %s %s %v", request.Method, request.Path, request.Body, request.Headers, request.Query, config)

	h := fnv.New32()
	_, err := h.Write([]byte(key))
	if err != nil {
		return nil, err
	}
	hashedKey := string(h.Sum(nil))

	if res, found := c.cache.Get(hashedKey); found {
		if err, ok := res.(*error); ok {
			return nil, *err
		}

		respAsBytes := res.(*[]byte)
		r := bufio.NewReader(bytes.NewReader(*respAsBytes))
		slog.Debug("return cached result", "method", request.Method, "host", config.Clusters[0].Cluster.Server, "path", request.Path)
		return http.ReadResponse(r, nil)
	}

	res, err := c.downstream.RequestApiServerRaw(request, config)
	if err != nil {
		c.cache.Set(hashedKey, &err, cache.DefaultExpiration)
		return nil, err
	}
	response, err := httputil.DumpResponse(res, true)
	if err != nil {
		return nil, err
	}
	c.cache.Set(hashedKey, &response, cache.DefaultExpiration)
	return res, nil
}

func (c cachingKube) RequestApiGroupsByCategory(config KubeConfig, category string) ([]v2beta1.APIGroupDiscovery, error) {
	key := fmt.Sprintf("resources %v %s", config, category)
	if res, found := c.cache.Get(key); found {
		if err, ok := res.(*error); ok {
			return nil, *err
		}

		return res.([]v2beta1.APIGroupDiscovery), nil
	}

	res, err := c.downstream.RequestApiGroupsByCategory(config, category)
	if err != nil {
		c.cache.Set(key, &err, cache.DefaultExpiration)
		return nil, err
	}
	c.cache.Set(key, res, cache.DefaultExpiration)
	return res, nil
}
