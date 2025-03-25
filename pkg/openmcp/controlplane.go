package openmcp

import (
	"fmt"
	"log/slog"

	"github.com/openmcp-project/ui-backend/pkg/k8s"
)

func GetControlPlaneKubeconfig(kube k8s.Kube, projectName, workspaceName, controlPlaneName, crateToken string, crateKubeconfig k8s.KubeConfig) (k8s.KubeConfig, error) {
	path := fmt.Sprintf("/apis/core.openmcp.cloud/v1alpha1/namespaces/project-%s--ws-%s/managedcontrolplanes/%s", projectName, workspaceName, controlPlaneName)

	cp := ControlPlane{}

	crateKubeconfig.SetUserToken(crateToken)

	err := k8s.RequestApiServer(kube, k8s.Request{
		Method: "GET",
		Path:   path,
	}, crateKubeconfig, &cp)
	if err != nil {
		return k8s.KubeConfig{}, err
	}
	if len(cp.Status.Components.Authentication.Access.Key) == 0 {
		return k8s.KubeConfig{}, fmt.Errorf("control-plane authentication key is empty")
	}

	secret := k8s.Secret{}
	path = fmt.Sprintf("api/v1/namespaces/%s/secrets/%s", cp.Status.Components.Authentication.Access.Namespace, cp.Status.Components.Authentication.Access.Name)
	err = k8s.RequestApiServer(kube, k8s.Request{
		Method: "GET",
		Path:   path,
	}, crateKubeconfig, &secret)
	if err != nil {
		return k8s.KubeConfig{}, err
	}

	data := secret.Data[cp.Status.Components.Authentication.Access.Key]

	if len(data) == 0 {
		return k8s.KubeConfig{}, fmt.Errorf("control-plane kubeconfig data is empty")
	}
	kubeconfig, err := k8s.ParseKubeconfig(string(data))
	if err != nil {
		slog.Error("failed to parse control-plane kubeconfig", "kubeconfig", cp.Status.Dataplane.Access.Kubeconfig, "err", err)
		return k8s.KubeConfig{}, err
	}

	return kubeconfig, nil
}

type ControlPlane struct {
	k8s.Resource
	Spec struct {
		Crossplane struct {
			Enabled bool   `json:"enabled"`
			Version string `json:"version"`
		} `json:"crossplane"`
		Dataplane struct {
			Gardener struct {
				Region string `json:"region"`
			} `json:"gardener"`
			Type string `json:"type"`
		} `json:"dataplane"`
	} `json:"spec"`
	Status struct {
		Components struct {
			Authentication struct {
				Access struct {
					Key       string `json:"key"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"access"`
			} `json:"authentication"`
		} `json:"components"`
		Conditions []struct {
			Type                string `json:"type"`
			Status              string `json:"status"`
			ObservedGenerations struct {
				ControlPlane          int `json:"controlPlane"`
				InternalConfiguration int `json:"internalConfiguration"`
				Resource              int `json:"resource"`
			} `json:"observedGenerations"`
		} `json:"conditions"`
		Dataplane struct {
			Access struct {
				Kubeconfig          string `json:"kubeconfig"`
				CreationTimestamp   string `json:"creationTimestamp"`
				ExpirationTimestamp string `json:"expirationTimestamp"`
			} `json:"access"`
			ObservedGeneration int    `json:"observedGeneration"`
			Status             string `json:"status"`
		} `json:"dataplane"`
	} `json:"status"`
}
