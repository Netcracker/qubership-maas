package v1

import "github.com/netcracker/qubership-maas/service/composite"

type RegistrationRequest struct {
	Id          string   `json:"id" validate:"required"`
	Namespaces  []string `json:"namespaces" validate:"required,gt=0,dive,lte=63"`
	ModifyIndex *uint64  `json:"modifyIndex"`
}

func (r RegistrationRequest) ToCompositeRegistration() *composite.CompositeRegistration {
	return &composite.CompositeRegistration{Id: r.Id, Namespaces: r.Namespaces, ModifyIndex: r.ModifyIndex}
}

type RegistrationResponse struct {
	Id          string   `json:"id"`
	Namespaces  []string `json:"namespaces"`
	ModifyIndex *uint64  `json:"modifyIndex"`
}

func NewRegistrationResponse(r *composite.CompositeRegistration) *RegistrationResponse {
	return &RegistrationResponse{Id: r.Id, Namespaces: r.Namespaces}
}
