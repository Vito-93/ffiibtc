package router

import (
	"fmt"
	"log"
	"net/http"
)

type Router struct {
	Mux *http.ServeMux
}

func NewRouter() *Router {
	return &Router{Mux: http.NewServeMux()}
}

func (r *Router) AddRoute(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.Mux.HandleFunc(pattern, handler)
}

func (r *Router) Run(port int) error {
	return http.ListenAndServe(fmt.Sprintf(":%d", port), r.logged())
}

func (r *Router) logged() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%s %s %s", req.RemoteAddr, req.Method, req.URL)
		r.Mux.ServeHTTP(w, req)
	})
}
