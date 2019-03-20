package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"text/template"
)

const (
	proxyHomepageTpl = `<h1>Welcome to RCOM Proxy Server</h1>
<a href="/">Go home</a>
<p>System status:</p>
<ul>
  <li>{{with .Mapping}} Proxy for env <b>{{ printf "%.6q" (index . "www.reuters.com")}}</b>
  {{else}}
  Proxy for unknow env
  {{end}}</li>
  <li>Server started at {{.StartTime}}</li>
</ul>

<p>You can:</p>
<ul>
  <li><a href="healthz">health check</a></li>
	<li><a href="switchhealthz">switch health status(Don't click! Test purpose only!)</a></li>
  {{if .Mapping}}<li>See <a href="mapping">host mapping</a></li>{{end}}
</ul>`

	mappingTpl = `<h1>Host Mapping</h1>
<a href="/">Go home</a>
<table>
{{range $k, $v := .Mapping}}
<tr><td>{{$k}}</td><td>{{$v}}</td>
</tr>
{{end}}
</table>
`
	healthzTpl = `{{.}}`
)

var (
	forceUnhealthyFlag   = false
	httpsHealthcheckHost = "wireapi.reuters.com"
	httpsHealthcheckPath = "/internalhealthcheck"
)

func (s *ProxyServer) ServeAdmin(w http.ResponseWriter, r *http.Request) {
	var tpl string
	var obj interface{} = *s
	statusCode := http.StatusOK
	switch path := r.URL.Path; path {
	case "/mapping":
		tpl = mappingTpl
	case "/healthz":
		tpl = healthzTpl
		if s.HttpsServer != nil {
			proxyUrl, _ := url.Parse("http://127.0.0.1" + s.Server.Addr)
			myClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
			resp, err := myClient.Get("https://" + httpsHealthcheckHost + httpsHealthcheckPath)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(resp.StatusCode)
				io.Copy(w, resp.Body)
				resp.Body.Close()
			}
		} else {
			tpl = ""
		}
		return
	case "/switchhealthz":
		s := fmt.Sprintf("Healthz %v -> %v", !forceUnhealthyFlag, forceUnhealthyFlag)
		logger.Printf(s)
		fmt.Fprintf(w, s)
		forceUnhealthyFlag = !forceUnhealthyFlag
		return
	case "/favicon.ico":
		tpl = ""
	default:
		tpl = proxyHomepageTpl
	}
	t, _ := template.New("admin").Parse(tpl)
	w.WriteHeader(statusCode)
	if err := t.Execute(w, obj); err != nil {
		logger.Printf("error: %v", err)
	}
}
