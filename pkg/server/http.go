package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	"smallbiznis-controlplane/pkg/config"

	"github.com/fsnotify/fsnotify"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

var ProvideHTTPServer = fx.Module("http.server",
	fx.Provide(NewHttpServer),
	fx.Invoke(Run),
)

type Server struct {
	server   *http.Server
	tlsMutex sync.RWMutex
	cert     *tls.Certificate
	certPath string
	keyPath  string
}

const (
	TenantID = "X-TENANT-ID"
)

func TenatIDAnnotator(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.New(nil)
	TenantID := req.Header.Get(TenantID)
	if TenantID != "" {
		md.Set(TenantID, TenantID)
	}
	return md
}

func RegisterServerMux() *runtime.ServeMux {
	return runtime.NewServeMux()
}

type Params struct {
	fx.In
	Config  *config.Config
	Handler *runtime.ServeMux
}

func NewHttpServer(p Params) *Server {
	cfg := p.Config
	srv := &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.Server.Addr),
			Handler:      p.Handler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		certPath: cfg.TLS.CertPath,
		keyPath:  cfg.TLS.KeyPath,
	}

	if cfg.TLS.Enable {
		srv.reloadCert() // initial load
		go srv.watchTLSFiles()

		srv.server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				srv.tlsMutex.RLock()
				defer srv.tlsMutex.RUnlock()

				if srv.cert == nil {
					return nil, fmt.Errorf("no TLS cert loaded")
				}

				return srv.cert, nil
			},
		}
	}

	return srv
}

// Reload TLS certificate
func (s *Server) reloadCert() {
	cert, err := tls.LoadX509KeyPair(s.certPath, s.keyPath)
	if err != nil {
		zap.L().Error("failed to reload TLS cert", zap.Error(err))
		return
	}
	s.tlsMutex.Lock()
	s.cert = &cert
	s.tlsMutex.Unlock()
	zap.L().Info("TLS certificate reloaded")
}

// Watch TLS cert/key file
func (s *Server) watchTLSFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		zap.L().Error("failed to create fsnotify watcher", zap.Error(err))
		return
	}
	defer watcher.Close()

	_ = watcher.Add(s.certPath)
	_ = watcher.Add(s.keyPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				s.reloadCert()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			zap.L().Error("watcher error", zap.Error(err))
		}
	}
}

func Run(lc fx.Lifecycle, srv *Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if srv.server.TLSConfig != nil {
				zap.L().Info("Starting HTTP server with mtls", zap.String("addr", srv.server.Addr))
				go srv.server.ListenAndServeTLS(srv.certPath, srv.keyPath)
			} else {
				zap.L().Info("Starting HTTP server with non mtls", zap.String("addr", srv.server.Addr))
				go srv.server.ListenAndServe()
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			zap.L().Info("Shutting down HTTP server gracefully...")
			return srv.server.Shutdown(ctx)
		},
	})
}
