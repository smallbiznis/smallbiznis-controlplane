package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("httpapi",
	fx.Invoke(registerHealthEndpoint),
)

func registerHealthEndpoint(mux *runtime.ServeMux) {
	if err := mux.HandlePath(http.MethodGet, "/healthz", func(ctx context.Context, w http.ResponseWriter, _ *http.Request, _ map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}); err != nil {
		zap.L().Error("failed to register health endpoint", zap.Error(err))
	}
}
