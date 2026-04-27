package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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

	jqConfig := server.JQConfig{
		MaxExpressionLength: getEnvInt("JQ_MAX_EXPRESSION_LENGTH", 500),
		ExecutionTimeout:    time.Duration(getEnvInt("JQ_EXECUTION_TIMEOUT_SECONDS", 5)) * time.Second,
		MaxResults:          getEnvInt("JQ_MAX_RESULTS", 10000),
	}

	mux := server.NewMiddleware(cachingKube, downstreamKube, jqConfig)

	address := ":3000"
	slog.Info("Starting server", "address", address)
	if err := http.ListenAndServe(address, mux); err != nil {
		slog.Error("failed to start server", "err", err)
	}
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
