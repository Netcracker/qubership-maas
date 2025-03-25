package v1

import (
	"fmt"
	_assert "github.com/stretchr/testify/assert"
	"maas/maas-service/model"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/validator"
	"testing"
)

func Test_ValidateBGNamespace(t *testing.T) {
	type fields struct {
		Namespace string
		Status    string
		Version   model.Version
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr _assert.ErrorAssertionFunc
	}{
		{name: "valid", fields: fields{Namespace: "test-ns", Status: "active", Version: "v1"}, wantErr: _assert.NoError},
		{name: "wrong namespace", fields: fields{Namespace: "", Status: "active", Version: "v1"}, wantErr: _assert.Error},
		{name: "wrong status", fields: fields{Namespace: "test-ns", Status: "my-status", Version: "v1"}, wantErr: _assert.Error},
		{name: "valid", fields: fields{Namespace: "test-ns", Status: "active", Version: ""}, wantErr: _assert.NoError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bgNamespace := &domain.BGNamespace{
				Name:    tt.fields.Namespace,
				State:   tt.fields.Status,
				Version: tt.fields.Version,
			}
			tt.wantErr(t, validator.Get().Struct(bgNamespace), fmt.Sprintf("validate BGNamespace(%v, %v, %v)", tt.fields.Namespace, tt.fields.Status, tt.fields.Version))
		})
	}
}
