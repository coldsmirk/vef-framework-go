package middleware

import (
	"path"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/middleware"
)

// spaEntryPath returns the SPA mount path, defaulting an empty path to "/".
func spaEntryPath(config *middleware.SPAConfig) string {
	if config.Path == "" {
		return "/"
	}

	return config.Path
}

// spaStaticPrefix returns the canonical static-asset prefix for a SPA mount:
// "/static/" for a root mount, "/app/static/" for an "/app" mount. path.Join
// normalizes the join so a root mount never yields a double-slashed "//static/".
func spaStaticPrefix(config *middleware.SPAConfig) string {
	return path.Join(spaEntryPath(config), "static") + "/"
}

type spaMiddleware struct {
	configs []*middleware.SPAConfig
}

func (*spaMiddleware) Name() string {
	return "spa"
}

func (*spaMiddleware) Order() int {
	return 1000
}

func (s *spaMiddleware) Apply(router fiber.Router) {
	for _, config := range s.configs {
		applySPA(router, config)
	}

	router.Use(func(ctx fiber.Ctx) error {
		if ctx.Method() == fiber.MethodGet {
			reqPath := ctx.Path()
			for _, config := range s.configs {
				entry := spaEntryPath(config)

				// Skip if already at SPA entry or static path to prevent infinite loop.
				if reqPath == entry || strings.HasPrefix(reqPath, spaStaticPrefix(config)) {
					continue
				}

				// Skip configured exclusions (e.g. "/api", "/ws") so the SPA
				// catch-all never swallows non-SPA routes.
				if hasAnyPrefix(reqPath, config.ExcludePaths) {
					continue
				}

				if strings.HasPrefix(reqPath, entry) {
					ctx.Path(entry)

					return ctx.RestartRouting()
				}
			}
		}

		return ctx.Next()
	})
}

func applySPA(router fiber.Router, config *middleware.SPAConfig) {
	entry := spaEntryPath(config)

	group := router.Group(
		entry,
		etag.New(etag.Config{Weak: true}),
		helmet.New(helmet.Config{
			XFrameOptions:             "sameorigin",
			ReferrerPolicy:            "no-referrer",
			XSSProtection:             "1; mode=block",
			CrossOriginEmbedderPolicy: "unsafe-none",
			CrossOriginOpenerPolicy:   "unsafe-none",
			CrossOriginResourcePolicy: "cross-origin",
			OriginAgentCluster:        "?1",
			ContentSecurityPolicy:     "default-src 'self'; img-src * data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self' ws: wss:; media-src 'self' blob:; object-src 'none'; worker-src 'self' blob:; frame-src 'self'",
		}),
	)

	group.Get("/", static.New("index.html", static.Config{
		FS:            config.Fs,
		CacheDuration: 30 * time.Second,
		Compress:      true,
	}))

	group.Get("/static/*", static.New("", static.Config{
		FS:            config.Fs,
		CacheDuration: 10 * time.Minute,
		MaxAge:        int((8 * time.Hour).Seconds()),
		Compress:      true,
		NotFoundHandler: func(ctx fiber.Ctx) error {
			ctx.Path(entry)

			return ctx.RestartRouting()
		},
	}))
}

func NewSPAMiddleware(configs []*middleware.SPAConfig) app.Middleware {
	if len(configs) == 0 {
		return nil
	}

	return &spaMiddleware{
		configs: configs,
	}
}

// hasAnyPrefix reports whether reqPath starts with any of the given non-empty prefixes.
func hasAnyPrefix(reqPath string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if prefix != "" && strings.HasPrefix(reqPath, prefix) {
			return true
		}
	}

	return false
}
