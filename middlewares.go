package achsvc

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
)

// Middleware describes a service (as opposed to endpoint) middleware.
type Middleware func(Service) Service

func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) NewOriginator(ctx context.Context, p Originator) (id ResourceID, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "NewOriginator", "id", p.ID, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.NewOriginator(ctx, p)
}

func (mw loggingMiddleware) Originator(ctx context.Context, id ResourceID) (p Originator, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "Originator", "id", id, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.Originator(ctx, id)
}

func (mw loggingMiddleware) Originators(ctx context.Context) []Originator {
	defer func(begin time.Time) {
		mw.logger.Log("method", "Originators", "took", time.Since(begin))
	}(time.Now())
	return mw.next.Originators(ctx)
}
