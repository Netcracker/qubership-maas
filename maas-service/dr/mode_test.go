package dr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsActive(t *testing.T) {
	assert.True(t, Active.IsActive())
	assert.False(t, Standby.IsActive())
}

func TestStringify(t *testing.T) {
	assert.Equal(t, "active", Active.String())
	assert.Equal(t, "standby", Standby.String())
	assert.Equal(t, "disabled", Disabled.String())
}

func TestFromString(t *testing.T) {
	assert.Equal(t, Active, ModeFromString("active"))
	assert.Equal(t, Standby, ModeFromString("standby"))
	assert.Equal(t, Disabled, ModeFromString("disabled"))
}
