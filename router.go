package chihttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	http2 "github.com/go-modulus/modulus/http"
	"github.com/go-modulus/modulus/http/errhttp"
)

func NewRouter(errorPipeline *errhttp.ErrorPipeline, config http2.ServeConfig) chi.Router {
	r := chi.NewRouter()
	r.MethodNotAllowed(
		errhttp.WrapHandler(
			errorPipeline,
			func(w http.ResponseWriter, req *http.Request) error {
				return http2.ErrMethodNotAllowed
			},
		),
	)
	r.NotFound(
		errhttp.WrapHandler(
			errorPipeline,
			func(w http.ResponseWriter, req *http.Request) error {
				return http2.ErrNotFound
			},
		),
	)
	if config.TTL > 0 {
		r.Use(chiMiddleware.Timeout(config.TTL))
	}
	if config.RequestSizeLimit > 0 {
		r.Use(chiMiddleware.RequestSize(int64(config.RequestSizeLimit.Bytes())))
	}
	return r
}
