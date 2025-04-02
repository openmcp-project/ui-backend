package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/openmcp-project/ui-backend/pkg/k8s"
)

type watchedKubeconfig struct {
	kubeconfig k8s.KubeConfig
	mu         sync.RWMutex
}

var crateKubeconfig watchedKubeconfig

func GetCrateKubeconfig() (k8s.KubeConfig, bool) {
	crateKubeconfig.mu.RLock()
	defer crateKubeconfig.mu.RUnlock()

	config := crateKubeconfig.kubeconfig
	if len(config.Clusters) == 0 {
		return config, false
	}

	deepCopiedUsers := make([]k8s.UserListEntry, len(config.Users))
	copy(deepCopiedUsers, config.Users)
	config.Users = deepCopiedUsers

	return config, true
}

func StartListeningOnKubeconfig(ctx context.Context, path string) {
	log.Println("Listening on kubeconfig file", path)

	// first time reading kubeconfig
	config, err := readKubeConfig(path)
	if err != nil {
		log.Fatalln("failed to read kubeconfig", err)
		return
	} else {
		crateKubeconfig.mu.Lock()
		crateKubeconfig.kubeconfig = config
		crateKubeconfig.mu.Unlock()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := watcher.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// starting the loop to check for changes in kubeconfig file
	go fileLoop(ctx, watcher, path)

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
		return
	}

	<-ctx.Done()
}

func fileLoop(ctx context.Context, watcher *fsnotify.Watcher, path string) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Fatalln("error:", err)
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Write) {
				config, err := readKubeConfig(path)
				if err != nil {
					log.Fatalln("failed to read kubeconfig", err)
					return
				} else {
					crateKubeconfig.mu.Lock()
					crateKubeconfig.kubeconfig = config
					crateKubeconfig.mu.Unlock()
				}
			}
		}
	}
}

func readKubeConfig(path string) (k8s.KubeConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return k8s.KubeConfig{}, err
	}

	kubeconfig, err := k8s.ParseKubeconfig(string(content))
	if err != nil {
		return k8s.KubeConfig{}, err
	}

	if len(kubeconfig.Clusters) == 0 {
		return k8s.KubeConfig{}, fmt.Errorf("kubeconfig for crate-cluster is invalid: .clusters is empty")
	}

	if len(kubeconfig.Users) == 0 {
		return k8s.KubeConfig{}, fmt.Errorf("kubeconfig for crate-cluster is invalid: .users is empty")
	}

	return kubeconfig, nil
}
