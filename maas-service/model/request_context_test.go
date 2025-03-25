package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAbsentRequestContext(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, RequestContextOf(ctx))
}

func TestWrongRequestContextValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	ctx := context.WithValue(context.Background(), requestContextKey, "other")
	RequestContextOf(ctx)
}

func TestRequestContextOf(t *testing.T) {
	rc := RequestContext{
		Namespace:    "core-dev",
		Microservice: "order-proc",
		Version:      "v2",
	}
	ctx := WithRequestContext(context.Background(), &rc)

	assert.Equal(t, &rc, RequestContextOf(ctx))
}
