package model

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getMinNumPartitionsFromRequest(t *testing.T) {
	type args struct {
		value interface{}
	}
	var zero int32 = 0
	var fortyTwo int32 = 42

	tests := []struct {
		name    string
		args    args
		want    *int32
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "nil", args: args{value: nil}, want: &zero, wantErr: assert.NoError},
		{name: "zero", args: args{value: zero}, want: &zero, wantErr: assert.NoError},
		{name: "empty", args: args{value: ""}, want: &zero, wantErr: assert.NoError},
		{name: "int_val", args: args{value: 42}, want: &fortyTwo, wantErr: assert.NoError},
		{name: "str_val", args: args{value: "42"}, want: &fortyTwo, wantErr: assert.NoError},
		{name: "wrong_val", args: args{value: "not digit"}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMinNumPartitionsFromRequest(tt.args.value)
			if !tt.wantErr(t, err, fmt.Sprintf("getMinNumPartitionsFromRequest(%v)", tt.args.value)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getMinNumPartitionsFromRequest(%v)", tt.args.value)
		})
	}
}

func TestTopicRegistrationReqDto_BuildTopicRegFromReq(t *testing.T) {
	dto := TopicRegistrationReqDto{
		Name:      "test-name",
		Versioned: false,
	}

	j, err := json.Marshal(&dto)
	assert.NoError(t, err)
	assert.Contains(t, string(j), "\"name\":\"test-name\"")
	assert.NotContains(t, string(j), "\"versioned\":false")

	dto = TopicRegistrationReqDto{
		Name:      "test-name",
		Versioned: true,
	}

	j, err = json.Marshal(&dto)
	assert.NoError(t, err)
	assert.Contains(t, string(j), "\"name\":\"test-name\"")
	assert.Contains(t, string(j), "\"versioned\":true")
}
