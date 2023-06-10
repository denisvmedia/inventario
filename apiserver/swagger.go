package apiserver

import (
	"github.com/go-chi/chi/v5"
	swagger "github.com/swaggo/http-swagger"
)

func Swagger() func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/swagger/*", swagger.Handler(
			swagger.URL("/swagger/doc.json"),
		))
	}
}
