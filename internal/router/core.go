package router

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/go-chi/chi/v5"
)

type Handler interface {
	RegisterRoute(r *chi.Mux)
	Handle(w http.ResponseWriter, r *http.Request)
}

func AsRoute(constructor any) any {
	return fx.Annotate(
		constructor,
		fx.As(new(Handler)),
		fx.ResultTags(`group:"handlers"`),
	)
}
