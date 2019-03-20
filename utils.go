package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func RewriteHTTP(rw http.ResponseWriter, r *http.Request, mapping *map[string]string) {
	transport := http.DefaultTransport

	outreq := new(http.Request)
	// Shallow copies of maps, like header
	*outreq = *r
	if cn, ok := rw.(http.CloseNotifier); ok {
		if requestCanceler, ok := transport.(requestCanceler); ok {
			// After the Handler has returned, there is no guarantee
			// that the channel receives a value, so to make sure
			reqDone := make(chan struct{})
			defer close(reqDone)
			clientGone := cn.CloseNotify()

			go func() {
				select {
				case <-clientGone:
					requestCanceler.CancelRequest(outreq)
				case <-reqDone:
				}
			}()
		}
	}

	// director
	// Map from original url to target url
	var targetHost string
	if mapping != nil {
		name, port := splitNamePort(r.Host)
		if m, ok := (*mapping)[name]; ok {
			if port != nil && r.URL.Host != "" {
				// This is from proxy, add port
				targetHost = fmt.Sprintf("%s:%s", m, *port)
			} else {
				// Access HTTPSServer directly, use 80 port
				targetHost = m
			}
		} else {
			targetHost = r.Host
		}
	} else {
		targetHost = r.Host
	}

	// Build output request
	target, err := url.Parse("http://" + targetHost)
	if err != nil {
		panic(err)
	}

	outreq.URL.Scheme = target.Scheme
	outreq.URL.Host = target.Host
	outreq.URL.Path = singleJoiningSlash(target.Path, outreq.URL.Path)

	// If Host is empty, the Request.Write method uses
	// the value of URL.Host.
	// force use URL.Host
	targetQuery := target.RawQuery
	if outreq.Host == "" {
		outreq.Host = r.URL.Host
	}

	if targetQuery == "" || outreq.URL.RawQuery == "" {
		outreq.URL.RawQuery = targetQuery + outreq.URL.RawQuery
	} else {
		outreq.URL.RawQuery = targetQuery + "&" + outreq.URL.RawQuery
	}

	if _, ok := outreq.Header["User-Agent"]; !ok {
		outreq.Header.Set("User-Agent", "")
	}

	// End of Director

	outreq.Close = false

	// We may modify the header (shallow copied above), so we only copy it.
	outreq.Header = make(http.Header)
	copyHeader(outreq.Header, r.Header)

	// Remove hop-by-hop headers listed in the "Connection" header, Remove hop-by-hop headers.
	removeHeaders(outreq.Header)

	// Add X-Forwarded-For Header.
	addXForwardedForHeader(outreq)

	res, err := transport.RoundTrip(outreq)
	if err != nil {
		logger.Printf("http: proxy error 1: %v\n", err)
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	// Remove hop-by-hop headers listed in the "Connection" header of the response, Remove hop-by-hop headers.
	removeHeaders(res.Header)

	// Copy header from response to client.
	copyHeader(rw.Header(), res.Header)

	// The "Trailer" header isn't included in the Transport's response, Build it up from Trailer.
	if len(res.Trailer) > 0 {
		trailerKeys := make([]string, 0, len(res.Trailer))
		for k := range res.Trailer {
			trailerKeys = append(trailerKeys, k)
		}
		rw.Header().Add("Trailer", strings.Join(trailerKeys, ", "))
	}

	rw.WriteHeader(res.StatusCode)
	if len(res.Trailer) > 0 {
		// Force chunking if we saw a response trailer.
		// This prevents net/http from calculating the length for short
		// bodies and adding a Content-Length.
		if fl, ok := rw.(http.Flusher); ok {
			fl.Flush()
		}
	}

	io.Copy(rw, res.Body)
	// close now, instead of defer, to populate res.Trailer
	res.Body.Close()
	copyHeader(rw.Header(), res.Trailer)
}

func ProxyHTTPS(rw http.ResponseWriter, req *http.Request, targetHost string) {
	rw.WriteHeader(http.StatusOK)
	hij, ok := rw.(http.Hijacker)
	if !ok {
		logger.Printf("http server does not support hijacker")
		return
	}

	clientConn, _, err := hij.Hijack()
	if err != nil {
		logger.Printf("http: proxy error 2: %v", err)
		return
	}

	var target string
	if targetHost != "" {
		target = targetHost
	} else {
		target = req.URL.Host
	}
	proxyConn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		logger.Printf("http: proxy error 3: %v, %v", err, target)
		return
	}

	// The returned net.Conn may have read or write deadlines
	// already set, depending on the configuration of the
	// Server, to set or clear those deadlines as needed
	// we set timeout to 5 minutes
	deadline := time.Now()
	deadline = deadline.Add(time.Minute * 1)

	err = clientConn.SetDeadline(deadline)
	if err != nil {
		logger.Printf("http: proxy error 4: %v", err)
		return
	}

	err = proxyConn.SetDeadline(deadline)
	if err != nil {
		logger.Printf("http: proxy error 5: %v", err)
		return
	}

	go transfer(clientConn, proxyConn)
	go transfer(proxyConn, clientConn)

	// go func() {
	// 	start1 := time.Now()
	// 	defer clientConn.Close()
	// 	defer proxyConn.Close()
	// 	io.Copy(clientConn, proxyConn)
	// 	logger.Printf("client->server cp close: %v s, %v", time.Now().Sub(start1), target)
	// }()
	//
	// start2 := time.Now()
	// defer clientConn.Close()
	// defer proxyConn.Close()
	// io.Copy(proxyConn, clientConn)
	// logger.Printf("server->client cp close: %v s, %v", time.Now().Sub(start2), target)
}

// -------------- Private --------------
func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func splitNamePort(s string) (name string, port *string) {
	ss := strings.SplitN(s, ":", 2)
	name = ss[0]
	if len(ss) > 1 {
		port = &ss[1]
	} else {
		port = nil
	}
	return
}

type requestCanceler interface {
	CancelRequest(req *http.Request)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func removeHeaders(header http.Header) {
	// Remove hop-by-hop headers listed in the "Connection" header.
	if c := header.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				header.Del(f)
			}
		}
	}

	// Hop-by-hop headers. These are removed when sent to the backend.
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
	var hopHeaders = []string{
		"Connection",
		"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",      // canonicalized version of "TE"
		"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
		"Transfer-Encoding",
		"Upgrade",
	}

	// Remove hop-by-hop headers
	for _, h := range hopHeaders {
		if header.Get(h) != "" {
			header.Del(h)
		}
	}
}

func addXForwardedForHeader(req *http.Request) {
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}
}

func FindRewrite(r *http.Request, mapping *map[string]string) string {
	if mapping != nil {
		name, _ := splitNamePort(r.Host)
		if v, ok := (*mapping)[name]; ok {
			return v
		}
	}
	return ""
}
