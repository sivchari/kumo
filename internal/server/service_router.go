package server

import "net/http"

type serviceRouter struct {
	router      *Router
	serviceName string
}

func (r serviceRouter) Handle(method, pattern string, handler http.HandlerFunc) {
	r.router.HandleWithService(method, pattern, r.serviceName, handler)
}

func (r serviceRouter) HandleFunc(method, pattern string, handler http.HandlerFunc) {
	r.Handle(method, pattern, handler)
}
