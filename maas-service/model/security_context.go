package model

import (
	"context"
	"log"
)

type SecurityContext struct {
	account                    *Account
	compositeIsolationDisabled bool
}

const (
	securityContextKey = "SecurityContext"
)

func NewSecurityContext(account *Account, compositeIsolationDisabled bool) *SecurityContext {
	return &SecurityContext{account: account, compositeIsolationDisabled: compositeIsolationDisabled}
}

func (sc *SecurityContext) UserHasRoles(expectedRoles ...RoleName) bool {
	if sc == nil || sc.account == nil {
		return false
	}
	return sc.account.HasRoles(expectedRoles...)
}

func (sc *SecurityContext) UserHasAccessToNamespace(namespace string) bool {
	if sc == nil || sc.account == nil {
		return false
	}
	return sc.account.HasAccessToNamespace(namespace)
}

func (sc *SecurityContext) IsCompositeIsolationDisabled() bool {
	return sc != nil && sc.compositeIsolationDisabled
}

func WithSecurityContext(ctx context.Context, securityContext *SecurityContext) context.Context {
	return context.WithValue(ctx, securityContextKey, securityContext)
}

func SecurityContextOf(ctx context.Context) *SecurityContext {
	v := ctx.Value(securityContextKey)
	if v != nil {
		if res, ok := v.(*SecurityContext); ok {
			return res
		} else {
			log.Panicf("Unexpected value `%+v' for reserved key: %v", v, securityContextKey)
		}
	}

	return nil
}
