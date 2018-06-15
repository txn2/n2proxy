package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"time"

	"github.com/txn2/n2proxy/rweng"
	"go.uber.org/zap"
)

const VERSION = "v1.1"

// Proxy defines the proxy handler see NewProx()
type Proxy struct {
	target  *url.URL
	proxy   *httputil.ReverseProxy
	cfgFile string
	logger  *zap.Logger
	eng     *rweng.Eng
}

//var _ http.RoundTripper = &transport{}

// NewProxy instances a new proxy server
func NewProxy(target string, cfgFile string, logger *zap.Logger) *Proxy {
	targetUrl, _ := url.Parse(target)

	// if cfgFile exists pass proxy
	eng, err := rweng.NewEngFromYml(cfgFile, logger)
	if err != nil {
		panic("Engine failure: " + err.Error())
	}

	pxy := httputil.NewSingleHostReverseProxy(targetUrl)

	proxy := &Proxy{
		target: targetUrl,
		proxy:  pxy,
		logger: logger,
		eng:    eng,
	}

	return proxy
}

// handle requests
func (p *Proxy) handle(w http.ResponseWriter, r *http.Request) {

	r.Header["n2proxy"] = []string{VERSION}
	start := time.Now()
	reqPath := r.URL.Path
	reqMethod := r.Method

	end := time.Now()
	latency := end.Sub(start)

	p.logger.Info(reqPath,
		zap.String("method", reqMethod),
		zap.String("path", reqPath),
		zap.String("time", end.Format(time.RFC3339)),
		zap.Duration("latency", latency),
	)

	r.Host = p.target.Host

	// process request
	p.eng.ProcessRequest(w, r)

	p.proxy.ServeHTTP(w, r)
}

// main function
func main() {
	port := getEnv("PORT", "9090")
	debug := getEnv("DEBUG", "true")
	cfg := getEnv("CFG", "")
	backend := getEnv("BACKEND", "http://example.com:80")

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err.Error())
	}

	if debug == "true" {
		logger, _ = zap.NewDevelopment()
	}

	logger.Info("Starting reverse proxy on port: " + port)
	logger.Info("Requests proxied to Backend: " + backend)

	// proxy
	proxy := NewProxy(backend, cfg, logger)

	// server
	http.HandleFunc("/", proxy.handle)
	http.ListenAndServe(":"+port, nil)
}

// getEnv gets an environment variable or sets a default if
// one does not exist.
func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}

	return value
}
