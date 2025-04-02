package k8s

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func ParseKubeconfig(kubeconfig string) (KubeConfig, error) {
	var config KubeConfig
	err := yaml.Unmarshal([]byte(kubeconfig), &config)
	if err != nil {
		return KubeConfig{}, fmt.Errorf("failed to parse kubeconfig: %v", err)
	}
	return config, nil
}

type KubeConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Clusters   []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users    []UserListEntry `yaml:"users"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster   string `yaml:"cluster"`
			User      string `yaml:"user"`
			Namespace string `yaml:"namespace"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	CurrentContext string `yaml:"current-context"`
}

type UserListEntry struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

type User struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
	Token                 string `yaml:"token"`
	Username              string `yaml:"username"`
	Password              string `yaml:"password"`
	Exec                  struct {
		ApiVersion string   `yaml:"apiVersion"`
		Args       []string `yaml:"args"`
		Command    string   `yaml:"command"`
	} `yaml:"exec"`
}

func (kc *KubeConfig) SetUserToken(token string) {
	if len(kc.Users) == 0 {
		kc.Users = append(kc.Users, struct {
			Name string `yaml:"name"`
			User User   `yaml:"user"`
		}{
			// Name is not used anywhere for now...
			Name: "default",
			User: User{
				Token: token,
			},
		})
	} else {
		for i := range kc.Users {
			kc.Users[i].User.ClientCertificateData = ""
			kc.Users[i].User.ClientKeyData = ""
			kc.Users[i].User.Username = ""
			kc.Users[i].User.Password = ""
			kc.Users[i].User.Token = token
		}
	}
}

type Secret struct {
	Data map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
}
