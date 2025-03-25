package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/openmcp-project/ui-backend/internal/utils"
	"github.com/openmcp-project/ui-backend/pkg/k8s"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openmcp-project/ui-backend/internal/server"
)

func main() {
	ctx := context.Background()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	kubeconfigPath := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	if kubeconfigPath == "" {
		slog.Error("env variable '%s' with kubeconfig path not set", clientcmd.RecommendedConfigPathEnvVar)
		return
	}
	go utils.StartListeningOnKubeconfig(ctx, kubeconfigPath)

	cachingKube := k8s.NewCachingKube(k8s.HttpKube{}, time.Second*30, time.Minute)
	downstreamKube := k8s.HttpKube{}

	mux := server.NewMiddleware(cachingKube, downstreamKube)

	address := ":3000"
	slog.Info("Starting server", "address", address)
	if err := http.ListenAndServe(address, mux); err != nil {
		slog.Error("failed to start server", "err", err)
	}
}
