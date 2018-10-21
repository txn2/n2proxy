package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/txn2/n2proxy/sec"

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
func NewProxy(target string, skpver bool, cfgFile string, logger *zap.Logger) *Proxy {
	targetUrl, err := url.Parse(target)
	if err != nil {
		fmt.Printf("Unable to parse URL: %s\n", err.Error())
		os.Exit(1)
	}

	// if cfgFile exists pass proxy
	eng, err := rweng.NewEngFromYml(cfgFile, logger)
	if err != nil {
		fmt.Printf("Engine failure: %s\n", err.Error())
		os.Exit(1)
	}

	pxy := httputil.NewSingleHostReverseProxy(targetUrl)

	if skpver {
		pxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // client uses self-signed cert
		}
	}

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
	cfgFileEnv := getEnv("CFG", "./cfg.yml")
	tlsCfgFileEnv := getEnv("TLSCFG", "")
	backendEnv := getEnv("BACKEND", "http://example.com:80")
	logoutEnv := getEnv("LOGOUT", "stdout")
	tlsEnvBool := false
	tlsEnv := getEnv("TLS", "false")
	if tlsEnv == "true" {
		tlsEnvBool = true
	}
	skpverEnvBool := false
	skpverEnv := getEnv("SKIP_VERIFY", "false")
	if skpverEnv == "true" {
		skpverEnvBool = true
	}
	crtEnv := getEnv("CRT", "./example.crt")
	keyEnv := getEnv("KEY", "./example.key")

	// command line falls back to env
	port := flag.String("port", portEnv, "port to listen on.")
	cfgFile := flag.String("cfg", cfgFileEnv, "config file path.")
	tlsCfgFile := flag.String("tlsCfg", tlsCfgFileEnv, "tls config file path.")
	backend := flag.String("backend", backendEnv, "backend server.")
	logout := flag.String("logout", logoutEnv, "log output stdout | ")
	srvtls := flag.Bool("tls", tlsEnvBool, "TLS Support (requires crt and key)")
	crt := flag.String("crt", crtEnv, "Path to cert. (enable --tls)")
	key := flag.String("key", keyEnv, "Path to private key. (enable --tls")
	skpver := flag.Bool("skip-verify", skpverEnvBool, "Skip backend tls verify.")
	version := flag.Bool("version", false, "Display version.")
	flag.Parse()

	if *version {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(1)
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.DisableCaller = true
	zapCfg.DisableStacktrace = true
	zapCfg.OutputPaths = []string{*logout}

	logger, err := zapCfg.Build()
	if err != nil {
		fmt.Printf("Can not build logger: %s\n", err.Error())
		return
	}

	err = logger.Sync()
	if err != nil {
		fmt.Printf("Error synchronizing logger: %s\n", err.Error())
		os.Exit(1)
	}

	logger.Info("Starting reverse proxy on port: " + *port)
	logger.Info("Requests proxied to Backend: " + *backend)

	// proxy
	proxy := NewProxy(*backend, *skpver, *cfgFile, logger)

	mux := http.NewServeMux()

	// server
	mux.HandleFunc("/", proxy.handle)

	srv := &http.Server{
		Addr:    ":" + *port,
		Handler: mux,
	}

	// If TLS is not specified serve the content unencrypted.
	if *srvtls != true {
		err = srv.ListenAndServe()
		if err != nil {
			fmt.Printf("Error starting proxy: %s\n", err.Error())
		}
		os.Exit(0)
	}

	// Get a generic TLS configuration
	tlsCfg := sec.GenericTLSConfig()

	if *tlsCfgFile == "" {
		logger.Warn("No TLS configuration specified, using default.")
	}

	if *tlsCfgFile != "" {
		logger.Info("Loading TLS configuration from " + *tlsCfgFile)
		tlsCfg, err = sec.NewTLSCfgFromYaml(*tlsCfgFile, logger)
		if err != nil {
			fmt.Printf("Error configuring TLS: %s\n", err.Error())
			os.Exit(0)
		}
	}

	logger.Info("Starting proxy in TLS mode.")

	srv.TLSConfig = tlsCfg
	srv.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0)

	err = srv.ListenAndServeTLS(*crt, *key)
	if err != nil {
		fmt.Printf("Error starting proxyin TLS mode: %s\n", err.Error())
	}
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
