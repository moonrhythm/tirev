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
)

func main() {
	var s *parapet.Server
	if front {
		s = parapet.NewFrontend()
	} else {
		s = parapet.New()
	}

	if !noHealthz {

		h := host.NewCIDR("0.0.0.0/0")
		l := location.Exact(healthzPath)
		l.Use(healthz.New())
		h.Use(l)

		s.Use(h)
	}
	if !noProm {
		s.Use(prom.Requests())
	}
	s.Use(gcp.HLBImmediateIP(0))

	if !noLog {
		s.Use(logger.Stdout())
	}

	if !noReqID {
		s.Use(requestid.New())
	}

	if ratelimitS > 0 {
		s.Use(ratelimit.FixedWindowPerSecond(ratelimitS))
	}
	if ratelimitM > 0 {
		s.Use(ratelimit.FixedWindowPerMinute(ratelimitM))
	}
	if ratelimitH > 0 {
		s.Use(ratelimit.FixedWindowPerMinute(ratelimitH))
	}

	if bodyLimitRequest > 0 {
		s.Use(body.LimitRequest(bodyLimitRequest))
	}
	if bodyBufferRequest {
		s.Use(body.BufferRequest())
	}

	if !noGzip {
		s.Use(compress.Gzip())
	}
	if !noBr {
		s.Use(compress.Br())
	}

	if redirectHTTPS {
		s.Use(redirect.HTTPS())
	}

	if hstsMode == "preload" {
		s.Use(hsts.Preload())
	} else if hstsMode != "" {
		s.Use(hsts.Default())
	}

	if redirectWWW == "www" {
		s.Use(redirect.WWW())
	} else if redirectWWW == "non" {
		s.Use(redirect.NonWWW())
	}

	var tr http.RoundTripper
	switch upstreamProto {
	default:
		tr = &transport.HTTP{}
	case "https":
		tr = &transport.HTTPS{}
	case "h2c":
		tr = &transport.H2C{}
	case "unix":
		tr = &transport.Unix{}
	}

	s.Use(upstream.SingleHost(upstreamAddr, tr))

	if !noProm {
		prom.Connections(s)
		prom.Networks(s)
		go prom.Start(fmt.Sprintf(":%d", promPort))
	}

	s.Addr = fmt.Sprintf(":%d", port)
	err := s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
