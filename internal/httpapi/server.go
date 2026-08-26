package httpapi

import (
	"net/http"
	"time"

	"timber-pest-remediation-ledger/internal/cases"
)

type API struct {
	service *cases.Service
	mux     *http.ServeMux
}

func New(service *cases.Service) *API {
	api := &API{service: service, mux: http.NewServeMux()}
	api.registerRoutes()
	return api
}

func (a *API) Handler() http.Handler {
	return securityHeaders(recoverMiddleware(a.mux))
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "服务处理请求时发生内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
