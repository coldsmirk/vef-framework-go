package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/middleware"
)

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
		applySpa(router, config)
	}

	router.Use(func(ctx fiber.Ctx) error {
		if ctx.Method() == fiber.MethodGet {
			path := ctx.Path()
			for _, config := range s.configs {
				// Skip if already at SPA entry or static path to prevent infinite loop
				if path == config.Path || strings.HasPrefix(path, config.Path+"/static/") {
					continue
				}

				if strings.HasPrefix(path, config.Path) {
					ctx.Path(config.Path)

					return ctx.RestartRouting()
				}
			}
		}

		return ctx.Next()
	})
}

func applySpa(router fiber.Router, config *middleware.SPAConfig) {
	group := router.Group(
		config.Path,
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

	fallbackPath := config.Path
	if fallbackPath == "" {
		fallbackPath = "/"
	}

	group.Get("/static/*", static.New("", static.Config{
		FS:            config.Fs,
		CacheDuration: 10 * time.Minute,
		MaxAge:        int((8 * time.Hour).Seconds()),
		Compress:      true,
		NotFoundHandler: func(ctx fiber.Ctx) error {
			ctx.Path(fallbackPath)

			return ctx.RestartRouting()
		},
	}))
}

func NewSPAMiddleware(configs []*middleware.SPAConfig) app.Middleware {
	if len(configs) == 0 {
		return nil
	}

	for _, config := range configs {
		if config.Path == "" {
			config.Path = "/"
		}
	}

	return &spaMiddleware{
		configs: configs,
	}
}
