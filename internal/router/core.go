package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

type Handler interface {
	RegisterRoute(r *chi.Mux)
	Handle(w http.ResponseWriter, r *http.Request)
}

const routeGroupTag = `group:"routes"`

func AsRoute(constructor any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.As(new(Handler)),
			fx.ResultTags(routeGroupTag),
		),
	)
}
