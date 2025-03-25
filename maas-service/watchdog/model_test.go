package watchdog

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStatus_Int(t *testing.T) {
	assert.Equal(t, 0, UNKNOWN.Int())
}
