package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/acoshift/configfile"
	"github.com/moonrhythm/parapet"
	"github.com/moonrhythm/parapet/pkg/body"
	"github.com/moonrhythm/parapet/pkg/compress"
	"github.com/moonrhythm/parapet/pkg/gcp"
	"github.com/moonrhythm/parapet/pkg/healthz"
	"github.com/moonrhythm/parapet/pkg/host"
	"github.com/moonrhythm/parapet/pkg/hsts"
	"github.com/moonrhythm/parapet/pkg/location"
	"github.com/moonrhythm/parapet/pkg/logger"
	"github.com/moonrhythm/parapet/pkg/prom"
	"github.com/moonrhythm/parapet/pkg/ratelimit"
	"github.com/moonrhythm/parapet/pkg/redirect"
	"github.com/moonrhythm/parapet/pkg/requestid"
	"github.com/moonrhythm/parapet/pkg/upstream"
	"github.com/moonrhythm/parapet/pkg/upstream/transport"
)

var config = configfile.NewEnvReader()

var (
	front             = config.Bool("front")
	port              = config.IntDefault("port", 8080)
	noHealthz         = config.Bool("no_healthz")
	healthzPath       = config.StringDefault("healthz_path", "/healthz")
	noProm            = config.Bool("no_prom")
	promPort          = config.IntDefault("prom_port", 9187)
	noGzip            = config.Bool("no_gzip")
	noBr              = config.Bool("no_br")
	noLog             = config.Bool("no_log")
	noReqID           = config.Bool("no_reqid")
	ratelimitS        = config.Int("ratelimit_s")
	ratelimitM        = config.Int("ratelimit_m")
	ratelimitH        = config.Int("ratelimit_h")
	bodyBufferRequest = config.Bool("body_bufferrequest")
	bodyLimitRequest  = config.Int64("body_limitrequest") // bytes
	redirectHTTPS     = config.Bool("redirect_https")
	hstsMode          = config.String("hsts")         // "", "preload", other = default
	redirectWWW       = config.String("redirect_www") // "", "www", "non"
	upstreamAddr      = config.String("upstream_addr")
	upstreamProto     = config.String("upstream_proto") // http, h2c, https, unix
	gcpHLB            = config.IntDefault("gcp_hlb", -1)
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

	if !noHealthz {
		h := host.NewCIDR("0.0.0.0/0")
		l := location.Exact(healthzPath)
		l.Use(healthz.New())
		h.Use(l)

		s.Use(h)
		fmt.Println("Registered healthz at", healthzPath)
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

	var tr http.RoundTripper
	switch upstreamProto {
	default:
		tr = &transport.HTTP{}
		fmt.Println("Using HTTP Transport")
	case "https":
		tr = &transport.HTTPS{}
		fmt.Println("Using HTTPS Transport")
	case "h2c":
		tr = &transport.H2C{}
		fmt.Println("Using H2C Transport")
	case "unix":
		tr = &transport.Unix{}
		fmt.Println("Using Unix Transport")
	}

	s.Use(upstream.SingleHost(upstreamAddr, tr))
	fmt.Println("Upstream", upstreamAddr)

	if !noProm {
		prom.Connections(s)
		prom.Networks(s)
		go prom.Start(fmt.Sprintf(":%d", promPort))
		fmt.Println("Starting prometheus on port", promPort)
	}

	s.Addr = fmt.Sprintf(":%d", port)
	fmt.Println("Starting parapet on port", port)
	fmt.Println()

	err := s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
