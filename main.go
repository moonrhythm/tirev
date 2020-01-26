package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/acoshift/configfile"
	"github.com/moonrhythm/parapet"
	"github.com/moonrhythm/parapet/pkg/body"
	"github.com/moonrhythm/parapet/pkg/compress"
	"github.com/moonrhythm/parapet/pkg/gcp"
	"github.com/moonrhythm/parapet/pkg/headers"
	"github.com/moonrhythm/parapet/pkg/healthz"
	"github.com/moonrhythm/parapet/pkg/hsts"
	"github.com/moonrhythm/parapet/pkg/logger"
	"github.com/moonrhythm/parapet/pkg/prom"
	"github.com/moonrhythm/parapet/pkg/ratelimit"
	"github.com/moonrhythm/parapet/pkg/redirect"
	"github.com/moonrhythm/parapet/pkg/requestid"
	"github.com/moonrhythm/parapet/pkg/upstream"
	"golang.org/x/crypto/acme/autocert"
)

var config = configfile.NewEnvReader()

var (
	front                = config.Bool("front")
	port                 = config.IntDefault("port", 8080)
	noHealthz            = config.Bool("no_healthz")
	healthzPath          = config.StringDefault("healthz_path", "/healthz")
	noProm               = config.Bool("no_prom")
	promPort             = config.IntDefault("prom_port", 9187)
	noGzip               = config.Bool("no_gzip")
	noBr                 = config.Bool("no_br")
	noLog                = config.Bool("no_log")
	noReqID              = config.Bool("no_reqid")
	reqHeaderSet         = parseHeaders(config.String("reqheader_set"))
	reqHeaderAdd         = parseHeaders(config.String("reqheader_add"))
	reqHeaderDel         = parseStrings(config.String("reqheader_del"))
	respHeaderSet        = parseHeaders(config.String("respheader_set"))
	respHeaderAdd        = parseHeaders(config.String("respheader_add"))
	respHeaderDel        = parseStrings(config.String("respheader_del"))
	ratelimitS           = config.Int("ratelimit_s")
	ratelimitM           = config.Int("ratelimit_m")
	ratelimitH           = config.Int("ratelimit_h")
	bodyBufferRequest    = config.Bool("body_bufferrequest")
	bodyLimitRequest     = config.Int64("body_limitrequest") // bytes
	redirectHTTPS        = config.Bool("redirect_https")
	hstsMode             = config.String("hsts")           // "", "preload", other = default
	redirectWWW          = config.String("redirect_www")   // "", "www", "non"
	upstreamAddr         = config.String("upstream_addr")  // comma split addr
	upstreamProto        = config.String("upstream_proto") // http, h2c, https, unix
	upstreamHeaderSet    = parseHeaders(config.String("upstream_header_set"))
	upstreamHeaderAdd    = parseHeaders(config.String("upstream_header_add"))
	upstreamHeaderDel    = parseStrings(config.String("upstream_header_del"))
	upstreamOverrideHost = config.String("upstream_override_host")
	upstreamPath         = config.String("upstream_path") // prefix path
	upstreamMaxIdleConns = config.IntDefault("upstream_maxidleconns", 32)
	gcpHLB               = config.IntDefault("gcp_hlb", -1)
	tlsKey               = config.String("tls_key")         // tls key file path
	tlsCert              = config.String("tls_cert")        // tls cert file path
	tlsMinVersion        = config.String("tls_min_version") // tls1.0, tls1.1, tls1.2, tls1.3
	autocertDir          = config.String("autocert_dir")
	autocertHosts        = config.String("autocert_hosts") // comma split hosts
)

func main() {
	fmt.Println("tirev")
	fmt.Println()

	var s *parapet.Server
	if front {
		s = parapet.NewFrontend()
		fmt.Println("Parapet Frontend Server")
	} else {
		s = parapet.New()
		fmt.Println("Parapet Server")
	}

	if autocertDir != "" && autocertHosts != "" {
		hosts := strings.Split(autocertHosts, ",")

		m := &autocert.Manager{
			Cache:      autocert.DirCache(autocertDir),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(hosts...),
		}
		s.TLSConfig = m.TLSConfig()
	}

	if tlsKey != "" && tlsCert != "" {
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			log.Fatal(err)
		}
		if s.TLSConfig == nil {
			s.TLSConfig = &tls.Config{}
		}
		s.TLSConfig.Certificates = append(s.TLSConfig.Certificates, cert)
	}

	if !noHealthz {
		h := &healthz.Healthz{
			Path: healthzPath,
		}
		h.Set(true)
		h.SetReady(true)
		s.Use(h)
		fmt.Println("Registered healthz at", healthzPath)
	}

	if len(reqHeaderSet) > 0 {
		s.Use(headers.SetRequest(reqHeaderSet...))
		fmt.Println("Registered Request Header Setter")
	}
	if len(reqHeaderAdd) > 0 {
		s.Use(headers.AddRequest(reqHeaderAdd...))
		fmt.Println("Registered Request Header Adder")
	}
	if len(reqHeaderDel) > 0 {
		s.Use(headers.DeleteRequest(reqHeaderDel...))
		fmt.Println("Registered Request Header Deleter")
	}
	if len(respHeaderSet) > 0 {
		s.Use(headers.SetResponse(respHeaderSet...))
		fmt.Println("Registered Response Header Setter")
	}
	if len(respHeaderAdd) > 0 {
		s.Use(headers.AddResponse(respHeaderAdd...))
		fmt.Println("Registered Response Header Adder")
	}
	if len(respHeaderDel) > 0 {
		s.Use(headers.DeleteResponse(respHeaderDel...))
		fmt.Println("Registered Response Header Deleter")
	}

	if !noProm {
		s.Use(prom.Requests())
	}

	if gcpHLB >= 0 {
		s.Use(gcp.HLBImmediateIP(gcpHLB))
		fmt.Println("Registered GCP HLB Immediate IP")
	}

	if !noLog {
		s.Use(logger.Stdout())
		fmt.Println("Registered Logger")
	}

	if !noReqID {
		s.Use(requestid.New())
		fmt.Println("Registered Request ID")
	}

	if ratelimitS > 0 {
		s.Use(ratelimit.FixedWindowPerSecond(ratelimitS))
		fmt.Println("Registered Ratelimiter (second):", ratelimitS)
	}
	if ratelimitM > 0 {
		s.Use(ratelimit.FixedWindowPerMinute(ratelimitM))
		fmt.Println("Registered Ratelimiter (minute):", ratelimitM)
	}
	if ratelimitH > 0 {
		s.Use(ratelimit.FixedWindowPerMinute(ratelimitH))
		fmt.Println("Registered Ratelimiter (hour):", ratelimitH)
	}

	if bodyLimitRequest > 0 {
		s.Use(body.LimitRequest(bodyLimitRequest))
		fmt.Println("Registered Request Body Limiter:", bodyLimitRequest)
	}
	if bodyBufferRequest {
		s.Use(body.BufferRequest())
		fmt.Println("Registered Request Body Bufferer")
	}

	if !noGzip {
		s.Use(compress.Gzip())
		fmt.Println("Registered Gzip Compressor")
	}
	if !noBr {
		s.Use(compress.Br())
		fmt.Println("Registered Br Compressor")
	}

	if redirectHTTPS {
		s.Use(redirect.HTTPS())
		fmt.Println("Registered HTTPS Redirector")
	}

	if hstsMode == "preload" {
		s.Use(hsts.Preload())
		fmt.Println("Registered HSTS Preload")
	} else if hstsMode != "" {
		s.Use(hsts.Default())
		fmt.Println("Registered HSTS")
	}

	if redirectWWW == "www" {
		s.Use(redirect.WWW())
		fmt.Println("Registered WWW Redirector")
	} else if redirectWWW == "non" {
		s.Use(redirect.NonWWW())
		fmt.Println("Registered Non-WWW Redirector")
	}

	if len(upstreamHeaderSet) > 0 {
		s.Use(headers.SetRequest(upstreamHeaderSet...))
		fmt.Println("Registered Upstream Header Setter")
	}
	if len(upstreamHeaderAdd) > 0 {
		s.Use(headers.AddRequest(upstreamHeaderAdd...))
		fmt.Println("Registered Upstream Header Adder")
	}
	if len(upstreamHeaderDel) > 0 {
		s.Use(headers.DeleteRequest(upstreamHeaderDel...))
		fmt.Println("Registered Upstream Header Deleter")
	}

	var tr http.RoundTripper
	switch upstreamProto {
	default:
		tr = &upstream.HTTPTransport{
			MaxIdleConns: upstreamMaxIdleConns,
		}
		fmt.Println("Using HTTP Transport")
	case "https":
		tr = &upstream.HTTPSTransport{
			MaxIdleConns: upstreamMaxIdleConns,
		}
		fmt.Println("Using HTTPS Transport")
	case "h2c":
		tr = &upstream.H2CTransport{}
		fmt.Println("Using H2C Transport")
	case "unix":
		tr = &upstream.UnixTransport{
			MaxIdleConns: upstreamMaxIdleConns,
		}
		fmt.Println("Using Unix Transport")
	}

	var targets []*upstream.Target
	for _, addr := range strings.Split(upstreamAddr, ",") {
		targets = append(targets, &upstream.Target{
			Host:      addr,
			Transport: tr,
		})
	}

	us := upstream.New(upstream.NewRoundRobinLoadBalancer(targets))
	us.Host = upstreamOverrideHost
	us.Path = upstreamPath
	s.Use(us)

	fmt.Println("Upstream", upstreamAddr)

	if !noProm {
		prom.Connections(s)
		prom.Networks(s)
		go prom.Start(fmt.Sprintf(":%d", promPort))
		fmt.Println("Starting prometheus on port", promPort)
	}

	if s.TLSConfig != nil {
		s.TLSConfig.MinVersion = parseTLSVersion(tlsMinVersion)
		fmt.Println("TLS Min Version", tlsMinVersion)
	}

	s.Addr = fmt.Sprintf(":%d", port)
	fmt.Println("Starting parapet on port", port)
	fmt.Println()

	err := s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func parseStrings(s string) []string {
	xs := strings.Split(s, ",")
	for i := range xs {
		xs[i] = strings.TrimSpace(xs[i])
	}
	return xs
}

func parseHeaders(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	ss := strings.Split(s, ",")

	var rs []string
	for _, x := range ss {
		ps := strings.Split(x, ":")
		if len(ps) != 2 {
			continue
		}
		rs = append(rs, strings.TrimSpace(ps[0]), strings.TrimSpace(ps[1]))
	}

	return rs
}

func parseTLSVersion(s string) uint16 {
	switch s {
	case "", "tls1.0":
		return tls.VersionTLS10
	case "tls1.1":
		return tls.VersionTLS11
	case "tls1.2":
		return tls.VersionTLS12
	case "tls1.3":
		return tls.VersionTLS13
	default:
		panic("invalid TLS version")
	}
}
