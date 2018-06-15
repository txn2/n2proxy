package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"time"

	"fmt"

	"flag"

	"github.com/txn2/n2proxy/rweng"
	"go.uber.org/zap"
)

var Version = "0.0.0"

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
	portEnv := getEnv("PORT", "9090")
	cfgEnv := getEnv("CFG", "./cfg.yml")
	backendEnv := getEnv("BACKEND", "http://example.com:80")

	// command line falls back to env
	port := flag.String("port", portEnv, "port to listen on.")
	cfg := flag.String("cfg", cfgEnv, "config file path.")
	backend := flag.String("backend", backendEnv, "backend server.")
	version := flag.Bool("version", false, "Display version.")
	flag.Parse()

	if *version {
		fmt.Printf("Version: %s\n", Version)
		return
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.DisableCaller = true
	zapCfg.DisableStacktrace = true
	zapCfg.OutputPaths = []string{"stdout", "test.log"}

	logger, err := zapCfg.Build()
	if err != nil {
		fmt.Printf("Can not build logger: %s\n", err.Error())
		return
	}

	logger.Sync()

	logger.Info("Starting reverse proxy on port: " + *port)
	logger.Info("Requests proxied to Backend: " + *backend)

	// proxy
	proxy := NewProxy(*backend, *cfg, logger)

	// server
	http.HandleFunc("/", proxy.handle)
	http.ListenAndServe(":"+*port, nil)
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
