package domain

import (
	"github.com/netcracker/qubership-maas/model"
	"time"
)

type BGState struct {
	//todo make required after migration
	ControllerNamespace string       `json:"controllerNamespace"`
	Origin              *BGNamespace `json:"originNamespace,omitempty" validate:"required"`
	Peer                *BGNamespace `json:"peerNamespace,omitempty" validate:"required"`
	UpdateTime          time.Time    `json:"updateTime" validate:"required"`
}

func (s *BGState) GetActiveNamespace() *BGNamespace {
	if s.Origin.State == "active" {
		return s.Origin
	}
	if s.Peer.State == "active" {
		return s.Peer
	}
	return nil
}

func (s *BGState) GetNotActiveNamespace() *BGNamespace {
	if s.Origin.State != "active" {
		return s.Origin
	}
	if s.Peer.State != "active" {
		return s.Peer
	}
	return nil
}

type BGNamespace struct {
	Name string `json:"name" validate:"required"`
	// FIXME make state strictly typed
	State   string        `json:"state" validate:"required,oneof=idle active candidate legacy"`
	Version model.Version `json:"version" validate:"omitempty,bgVersion"`
}

type BGNamespaces struct {
	Origin              string `json:"originNamespace" validate:"required"`
	Peer                string `json:"peerNamespace" validate:"required"`
	ControllerNamespace string `json:"controllerNamespace"`
}

func (n *BGNamespaces) IsMember(namespace string) bool {
	return namespace == n.ControllerNamespace ||
		namespace == n.Origin ||
		namespace == n.Peer
}
