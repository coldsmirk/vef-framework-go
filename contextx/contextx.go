package contextx

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/security"
)

type contextKey int

const (
	KeyRequest contextKey = iota
	KeyRequestID
	KeyRequestIP
	KeyPrincipal
	KeyLogger
	KeyDB
	KeyDataPermApplier
)

// setValue stores a value in the context, handling both fiber.Ctx and standard context.Context.
func setValue[T any](ctx context.Context, key contextKey, value T) context.Context {
	if c, ok := ctx.(fiber.Ctx); ok {
		c.Locals(key, value)

		return c
	}

	return context.WithValue(ctx, key, value)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(KeyRequestID).(string)

	return id
}

func SetRequestID(ctx context.Context, requestID string) context.Context {
	return setValue(ctx, KeyRequestID, requestID)
}

func Principal(ctx context.Context) *security.Principal {
	principal, _ := ctx.Value(KeyPrincipal).(*security.Principal)

	return principal
}

func SetPrincipal(ctx context.Context, principal *security.Principal) context.Context {
	return setValue(ctx, KeyPrincipal, principal)
}

func Logger(ctx context.Context, fallbacks ...log.Logger) log.Logger {
	if logger, ok := ctx.Value(KeyLogger).(log.Logger); ok {
		return logger
	}

	for _, fallback := range fallbacks {
		if reflectx.IsNotEmpty(fallback) {
			return fallback
		}
	}

	return nil
}

func SetLogger(ctx context.Context, logger log.Logger) context.Context {
	return setValue(ctx, KeyLogger, logger)
}

func DB(ctx context.Context, fallbacks ...orm.DB) orm.DB {
	if db, ok := ctx.Value(KeyDB).(orm.DB); ok {
		return db
	}

	for _, fallback := range fallbacks {
		if reflectx.IsNotEmpty(fallback) {
			return fallback
		}
	}

	return nil
}

func SetDB(ctx context.Context, db orm.DB) context.Context {
	return setValue(ctx, KeyDB, db)
}

func DataPermApplier(ctx context.Context) security.DataPermissionApplier {
	applier, _ := ctx.Value(KeyDataPermApplier).(security.DataPermissionApplier)

	return applier
}

func SetDataPermApplier(ctx context.Context, applier security.DataPermissionApplier) context.Context {
	return setValue(ctx, KeyDataPermApplier, applier)
}

func RequestIP(ctx context.Context) string {
	ip, _ := ctx.Value(KeyRequestIP).(string)

	return ip
}

func SetRequestIP(ctx context.Context, ip string) context.Context {
	return setValue(ctx, KeyRequestIP, ip)
}
