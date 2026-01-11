package fx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

func RegisterHTTPServerLifecycle(
	lc fx.Lifecycle,
	srv *http.Server,
	log *zap.SugaredLogger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Infow("http server starting", "addr", srv.Addr)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Errorw("http server crashed", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Infow("http server stopping", "addr", srv.Addr)
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}
