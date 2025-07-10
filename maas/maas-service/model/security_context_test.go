package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSecurityContextNil(t *testing.T) {
	ctx := context.Background()

	empty := SecurityContextOf(ctx)
	assert.False(t, empty.UserHasRoles(ManagerRole))
	assert.False(t, empty.IsCompositeIsolationDisabled())
}

func TestSecurityContext(t *testing.T) {
	ctx := WithSecurityContext(
		context.Background(),
		NewSecurityContext(&Account{
			Username: "scott",
			Roles:    []RoleName{ManagerRole},
		}, true),
	)

	// test
	secCtx := SecurityContextOf(ctx)
	assert.NotNil(t, secCtx)
	assert.True(t, secCtx.UserHasRoles(ManagerRole))
	assert.True(t, secCtx.IsCompositeIsolationDisabled())
}
