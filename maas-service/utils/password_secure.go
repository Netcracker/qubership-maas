package utils

import (
	"fmt"
)

const (
	ENCODER_PREFIX_SEP = ":"
	ENCODER_PLAIN      = "plain"
)

func SecurePassword(password string) string {
	return fmt.Sprintf("%s%s%s", ENCODER_PLAIN, ENCODER_PREFIX_SEP, password)
}
