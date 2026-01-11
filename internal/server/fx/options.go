package fx

import (
	"context"
	"net"
	"net/http"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/server"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var ServerOptions = fx.Options(
	fx.Provide(server.NewHTTPServer),
	fx.Invoke(registerLifecycleHooks),
)

type hooksParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    config.Config
	Logger    *zap.Logger
	Server    *http.Server
}

func registerLifecycleHooks(p hooksParams) {
	var ln net.Listener

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Logger.Info("http_server_starting", zap.String("addr", p.Server.Addr))

			var err error
			ln, err = net.Listen("tcp", p.Server.Addr)
			if err != nil {
				return err
			}

			go func() {
				err := p.Server.Serve(ln)
				if err != nil && err != http.ErrServerClosed {
					p.Logger.Error("http_server_listen_failed", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Info("http_server_stopping")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			return p.Server.Shutdown(shutdownCtx)
		},
	})
}
