package dao

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestTxToString(t *testing.T) {
	_, tx1 := newTransactionContext(context.Background())

	re := regexp.MustCompile("^tx#\\d+:\\d+$")
	assert.True(t, re.MatchString(fmt.Sprintf("%s", tx1)))
}
