package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBGStateEntity_checkEqual(t *testing.T) {
	one := BGStateEntity{
		ID:         1,
		BgDomainId: 1,
		OriginNs: BGNamespace{
			Name:    "a",
			State:   "active",
			Version: "v1",
		},
		PeerNs: BGNamespace{
			Name:    "b",
			State:   "candidate",
			Version: "v2",
		},
		UpdateTime: time.Now(),
	}

	other := BGStateEntity{
		ID:         1,
		BgDomainId: 1,
		OriginNs: BGNamespace{
			Name:    "c",
			State:   "active",
			Version: "v1",
		},
		PeerNs: BGNamespace{
			Name:    "e",
			State:   "candidate",
			Version: "v2",
		},
		UpdateTime: time.Now(),
	}

	assert.True(t, one.checkEqual(one))
	assert.False(t, one.checkEqual(other))
}
