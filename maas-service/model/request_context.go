package model

import (
	"context"
	"log"
)

const (
	requestContextKey = "RequestContext"
)

type RequestContext struct {
	Namespace    string
	Microservice string
	Version      Version
}

func WithRequestContext(ctx context.Context, requestContext *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey, requestContext)
}

func RequestContextOf(ctx context.Context) *RequestContext {
	v := ctx.Value(requestContextKey)
	if v != nil {
		if res, ok := v.(*RequestContext); ok {
			return res
		} else {
			log.Panicf("Unexpected value `%+v' for reserved key: %v", v, requestContextKey)
		}
	}

	return nil
}
