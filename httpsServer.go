package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func HelloServer(w http.ResponseWriter, req *http.Request) {
	log.Println("request in")
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
}

type HTTPSServer struct {
	server  *http.Server
	pemPath string
	keyPath string
	mapping *map[string]string
}

func NewHTTPSServer(port string, pemPath string, keyPath string, mapping *map[string]string) *HTTPSServer {
	srv := http.Server{
		Addr:         ":" + port,
		ErrorLog:     logger,
		ReadTimeout:  25 * time.Second,
		WriteTimeout: 25 * time.Second,
		IdleTimeout:  30 * time.Second,
		// Disable HTTP/2.
		// TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	s := &HTTPSServer{server: &srv, pemPath: pemPath, keyPath: keyPath, mapping: mapping}
	s.server.Handler = http.HandlerFunc(s.handlerHTTPS)
	return s
}

func (s *HTTPSServer) Serve() {
	logger.Println("HTTPS listens on " + s.server.Addr)
	if err := s.server.ListenAndServeTLS(s.pemPath, s.keyPath); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen https on %v: %v\n", s.server.Addr, err)
	}
}

func (s *HTTPSServer) Shutdown(ctx context.Context) error {
	logger.Println("HTTPS shutdown")
	ss := s.server
	ss.SetKeepAlivesEnabled(false)
	if err := ss.Shutdown(ctx); err != nil {
		logger.Printf("Could not gracefully shutdown the https server: %v\n", err)
		return err
	}
	return nil
}

func (s *HTTPSServer) handlerHTTPS(w http.ResponseWriter, r *http.Request) {
	logger.Printf("request %v -> [%v] [%v] https://%v%v", r.RemoteAddr, r.Method, r.Proto, r.Host, r.URL)
	RewriteHTTP(w, r, s.mapping)
}
