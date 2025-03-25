package validator

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	_assert "github.com/stretchr/testify/assert"
	"maas/maas-service/model"
	"reflect"
	"testing"
)

func Test_ValidateRabbit(t *testing.T) {
	assert := _assert.New(t)
	ri := &model.RabbitInstance{
		ApiUrl:   "http://localhost:15672/",
		AmqpUrl:  "amqp://localhost:5672/",
		Password: "plain: password",
	}
	assert.NoError(Get().Struct(ri))

	assert.Equal("http://localhost:15672", ri.ApiUrl)
	assert.Equal("amqp://localhost:5672", ri.AmqpUrl)

	ri = &model.RabbitInstance{
		ApiUrl:   "https://localhost:15672",
		AmqpUrl:  "amqps://localhost:5672",
		Password: "plain: password",
	}
	assert.NoError(Get().Struct(ri))

	ri = &model.RabbitInstance{
		ApiUrl:   "wrong://localhost:15672",
		AmqpUrl:  "wrong://localhost:5672",
		Password: "no prefix password",
	}

	err := Get().Struct(ri)
	assert.NotNil(err)
	validationErrors := err.(validator.ValidationErrors)
	assert.Equal(2, len(validationErrors))
	assert.Equal(HttpOrHttps, validationErrors[0].Tag())
	assert.Equal(AmqpOrAmqps, validationErrors[1].Tag())
}

func Test_ValidateKafka(t *testing.T) {
	assert := _assert.New(t)
	ki := &model.KafkaInstance{
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"addr:1234"}},
		MaasProtocol: model.Plaintext,
		Credentials: map[model.KafkaRole][]model.KafkaCredentials{
			model.Admin:  {{AuthType: model.PlainAuth}},
			model.Client: {{AuthType: model.PlainAuth}},
		},
	}
	assert.NoError(Get().Struct(ki))

	ki = &model.KafkaInstance{
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"addr:1234"}},
		MaasProtocol: "wrongProtocol",
		Credentials: map[model.KafkaRole][]model.KafkaCredentials{model.Admin: {{
			AuthType: "wrongType",
		}}},
	}

	err := Get().Struct(ki)
	assert.NotNil(err)
	validationErrors := err.(validator.ValidationErrors)
	assert.Equal(2, len(validationErrors))
	assert.Equal(KafkaProtocol, validationErrors[0].Tag())
	assert.Equal(KafkaCredentialsRoles, validationErrors[1].Tag())

	ki = &model.KafkaInstance{
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"addr:1234"}},
		MaasProtocol: model.Plaintext,
	}

	err = Get().Struct(ki)
	assert.NoError(err)
}

func TestClassifierNormalizationErrorForNameField(t *testing.T) {
	assert := _assert.New(t)
	assert.Error(Get().Var(model.Classifier{Name: ""}, ClassifierTag))
}

func TestClassifierNormalizationErrorForEmptyNamespaceField(t *testing.T) {
	assert := _assert.New(t)
	assert.Error(Get().Var(model.Classifier{Name: "cde"}, ClassifierTag))
}

func TestClassifierNormalizationDefaultNamespace(t *testing.T) {
	assert := _assert.New(t)

	actual := model.Classifier{Name: "abc", Namespace: "cde"}
	assert.NoError(Get().Var(actual, ClassifierTag))
	expected := model.Classifier{Name: "abc", Namespace: "cde"}
	assert.Equal(actual, expected)
}

func TestFormatErrors(t *testing.T) {
	type args struct {
		ve validator.ValidationErrors
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "nil", args: struct{ ve validator.ValidationErrors }{ve: nil}, want: nil},
		{name: "unknown", args: struct{ ve validator.ValidationErrors }{ve: []validator.FieldError{FieldError{
			TagVal:   "unknown",
			FieldVal: "FieldVal",
			ParamVal: "ParamVal",
		}}}, want: []string{"error"}},
		{name: ExcludedWithTag, args: struct{ ve validator.ValidationErrors }{ve: []validator.FieldError{FieldError{
			TagVal:   ExcludedWithTag,
			FieldVal: "FieldVal",
			ParamVal: "ParamVal",
		}}}, want: []string{"FieldVal used with ParamVal. FieldVal and ParamVal is mutual exclusive, you can use only one at a time"}},
		{name: RequiredTag, args: struct{ ve validator.ValidationErrors }{ve: []validator.FieldError{FieldError{
			TagVal:   RequiredTag,
			FieldVal: "FieldVal",
			ParamVal: "ParamVal",
		}}}, want: []string{"FieldVal is required"}},
		{name: ClassifierTag, args: struct{ ve validator.ValidationErrors }{ve: []validator.FieldError{FieldError{
			TagVal:   ClassifierTag,
			FieldVal: "FieldVal",
			ParamVal: "ParamVal",
		}}}, want: []string{"classifier must contain 'name' and 'namespace' fields"}},
		{name: "long_path", args: struct{ ve validator.ValidationErrors }{ve: []validator.FieldError{FieldError{
			TagVal:       RequiredTag,
			FieldVal:     "FieldVal",
			ParamVal:     "ParamVal",
			NamespaceVal: "MyStruct.Tenant.Classifier",
		}}}, want: []string{"tenant.classifier is required"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_assert.Equalf(t, tt.want, FormatErrors(tt.args.ve), "FormatErrors(%v)", tt.args.ve)
		})
	}
}

type FieldError struct {
	TagVal       string
	FieldVal     string
	ParamVal     string
	NamespaceVal string
}

func (f FieldError) Tag() string {
	return f.TagVal
}

func (f FieldError) ActualTag() string {
	panic("implement me")
}

func (f FieldError) Namespace() string {
	if f.NamespaceVal == "" {
		return f.FieldVal
	}
	return f.NamespaceVal
}

func (f FieldError) StructNamespace() string {
	panic("implement me")
}

func (f FieldError) Field() string {
	return f.FieldVal
}

func (f FieldError) StructField() string {
	panic("implement me")
}

func (f FieldError) Value() interface{} {
	panic("implement me")
}

func (f FieldError) Param() string {
	return f.ParamVal
}

func (f FieldError) Kind() reflect.Kind {
	panic("implement me")
}

func (f FieldError) Type() reflect.Type {
	panic("implement me")
}

func (f FieldError) Translate(_ ut.Translator) string {
	panic("implement me")
}

func (f FieldError) Error() string {
	return "error"
}
