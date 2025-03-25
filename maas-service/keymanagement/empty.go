package keymanagement

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var log = logging.GetLogger("key-manager:plain")

const (
	ENCODER_PLAIN      = "plain"
	ENCODER_PREFIX_SEP = ":"
)

type PlainKeyManager struct {
}

func NewPlain() *PlainKeyManager {
	return &PlainKeyManager{}
}

func (PlainKeyManager) SecurePassword(ctx context.Context, namespace, password, client string) (string, error) {
	log.WarnC(ctx, "Encode password in plain mode")
	return fmt.Sprintf("%s%s%s", ENCODER_PLAIN, ENCODER_PREFIX_SEP, password), nil
}

func (PlainKeyManager) DeletePassword(_ context.Context, key string) error {
	return nil
}
