package controller

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_asPrometheusLabels(t *testing.T) {
	type testEntity struct {
		First  string
		Second int
	}
	type testEntityWithCustomLabel struct {
		First  string `prometheus_label:"first-LBL"`
		Second int    `prometheus_label:"-"`
	}
	type args[T any] struct {
		entity T
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want string
	}
	tests := []testCase[any]{
		{name: "no_custom_label", args: struct{ entity any }{entity: testEntity{
			First:  "first-value",
			Second: 42,
		}}, want: "first=\"first-value\", second=\"42\""},
		{name: "no_custom_label_empty_value", args: struct{ entity any }{entity: testEntity{
			First:  "",
			Second: 42,
		}}, want: "first=\"\", second=\"42\""},
		{name: "custom_label", args: struct{ entity any }{entity: testEntityWithCustomLabel{
			First:  "first-value",
			Second: 42,
		}}, want: "first-LBL=\"first-value\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, asPrometheusLabels(&tt.args.entity), "asPrometheusLabels(%v)", tt.args.entity)
		})
	}
}
