package cleanup

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNamespaceCleanupService_DeleteNamespace(t *testing.T) {
	counter := 0
	cb := func(ctx context.Context, namespace string) error { counter += 1; return nil }
	nc := NewNamespaceCleanupService(cb, cb)
	nc.DeleteNamespace(context.Background(), "cloud-dev")
	assert.Equal(t, 2, counter)
}

func TestNamespaceCleanupService_DeleteNamespaceWithError(t *testing.T) {
	counter := 0
	cb := func(ctx context.Context, namespace string) error { counter += 1; return nil }
	cb_err := func(ctx context.Context, namespace string) error { return errors.New("oops") }
	nc := NewNamespaceCleanupService(cb, cb_err, cb)
	assert.NotNil(t, nc.DeleteNamespace(context.Background(), "cloud-dev"))
	assert.Equal(t, 1, counter)
}
