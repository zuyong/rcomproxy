package main

import (
	"context"
	"net/http"
	"time"
)

type ProxyServer struct {
	Server      *http.Server
	Mapping     *map[string]string
	HttpsServer *HTTPSServer
	StartTime   time.Time
}

func NewProxyServer(proxyPort string, httpsPort string, pemPath string, keyPath string, mapping *map[string]string) *ProxyServer {
	srv := http.Server{
		Addr:         ":" + proxyPort,
		ErrorLog:     logger,
		ReadTimeout:  25 * time.Second,
		WriteTimeout: 25 * time.Second,
		IdleTimeout:  30 * time.Second,
		// Disable HTTP/2.
		// TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	s := &ProxyServer{Server: &srv, Mapping: mapping}

	if pemPath != "" && keyPath != "" && httpsPort != "" && mapping != nil && len(*mapping) > 0 {
		s.HttpsServer = NewHTTPSServer(httpsListenPort, pemPath, keyPath, mapping)
	}

	s.Server.Handler = http.HandlerFunc(s.handlerHTTP)
	return s
}

func (s *ProxyServer) Serve() {
	s.StartTime = time.Now()
	if s.HttpsServer != nil {
		go s.HttpsServer.Serve()
	}

	logger.Println("Proxy listens on " + s.Server.Addr)
	if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen proxy on %v: %v\n", s.Server.Addr, err)
	}
}

func (s *ProxyServer) Shutdown(ctx context.Context) error {
	logger.Println("Proxy server is shutting down...")
	ss := s.Server
	ss.SetKeepAlivesEnabled(false)

	if s.HttpsServer != nil {
		_ = s.HttpsServer.Shutdown(ctx)
	}

	logger.Println("Proxy shutdown")
	if err := ss.Shutdown(ctx); err != nil {
		logger.Printf("Could not gracefully shutdown the proxy server: %v\n", err)
		return err
	}
	return nil
}

// ---------- private ----------
func (s *ProxyServer) handlerHTTP(w http.ResponseWriter, r *http.Request) {
	m := FindRewrite(r, s.Mapping)
	if r.Host != adminDomain {
		if m != "" {
			logger.Printf("Rewrite %v -> [%v] [%v] %v -> %v", r.RemoteAddr, r.Method, r.Proto, r.URL, m)
		} else {
			logger.Printf("Forward %v -> [%v] [%v] %v", r.RemoteAddr, r.Method, r.Proto, r.URL)
		}
	}

	if r.Method == "CONNECT" {
		if s.HttpsServer == nil || m == "" {
			ProxyHTTPS(w, r, "")
		} else {
			ProxyHTTPS(w, r, "127.0.0.1"+s.HttpsServer.server.Addr)
		}
	} else if r.Host == adminDomain {
		s.ServeAdmin(w, r)
	} else {
		RewriteHTTP(w, r, s.Mapping)
	}
}
