package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBGNamespaces_IsMember(t *testing.T) {
	type fields struct {
		Origin              string
		Peer                string
		ControllerNamespace string
	}
	type args struct {
		namespace string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{name: "empty namespace", fields: fields{
			Origin:              "origin",
			Peer:                "peer",
			ControllerNamespace: "controller",
		}, args: args{namespace: ""}, want: false},
		{name: "origin member", fields: fields{
			Origin:              "origin",
			Peer:                "peer",
			ControllerNamespace: "controller",
		}, args: args{namespace: "origin"}, want: true},
		{name: "peer member", fields: fields{
			Origin:              "origin",
			Peer:                "peer",
			ControllerNamespace: "controller",
		}, args: args{namespace: "peer"}, want: true},
		{name: "controller member", fields: fields{
			Origin:              "origin",
			Peer:                "peer",
			ControllerNamespace: "controller",
		}, args: args{namespace: "controller"}, want: true},
		{name: "not member", fields: fields{
			Origin:              "origin",
			Peer:                "peer",
			ControllerNamespace: "controller",
		}, args: args{namespace: "not-member"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &BGNamespaces{
				Origin:              tt.fields.Origin,
				Peer:                tt.fields.Peer,
				ControllerNamespace: tt.fields.ControllerNamespace,
			}
			assert.Equalf(t, tt.want, n.IsMember(tt.args.namespace), "IsMember(%v)", tt.args.namespace)
		})
	}
}
