package validator

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"regexp"
	"strings"
	"sync"
)

const (
	ClassifierTag         = "classifier"
	RequiredTag           = "required"
	ExcludedWithTag       = "excluded_with"
	AmqpOrAmqps           = "amqp"
	HttpOrHttps           = "http"
	KafkaProtocol         = "kafkaProtocol"
	PlainPassword         = "plainPassword"
	KafkaAuth             = "kafkaAuth"
	KafkaCredentialsRoles = "kafkaCredentialsRoles"
	BGVersion             = "bgVersion"
)

var (
	validBgVersion = regexp.MustCompile(`^[V,v]\d+$`)
	instance       *validator.Validate
	once           sync.Once
)

func Get() *validator.Validate {
	once.Do(func() {
		instance = validator.New()
		err := instance.RegisterValidation(ClassifierTag, classifierValidation)
		if err != nil {
			panic(err)
		}
		err = instance.RegisterValidation("removeTrailingSlashes", removeTrailingSlashes)
		if err != nil {
			panic(err)
		}
		instance.RegisterAlias(AmqpOrAmqps, "startswith=amqp://|startswith=amqps://")
		instance.RegisterAlias(HttpOrHttps, "startswith=http://|startswith=https://")
		instance.RegisterAlias(KafkaProtocol, fmt.Sprintf("oneof=%s %s %s %s", model.Plaintext, model.SaslPlaintext, model.Ssl, model.SaslSsl))
		instance.RegisterAlias(PlainPassword, fmt.Sprintf("startswith=%s", model.Plain))
		instance.RegisterAlias(KafkaAuth, fmt.Sprintf("oneof=%s %s %s %s %s",
			model.PlainAuth, model.SslCertAuth, model.SCRAMAuth, model.SslCertPlusPlain, model.SslCertPlusSCRAM))

		err = instance.RegisterValidation(KafkaCredentialsRoles, kafkaCredentialsRoles)
		if err != nil {
			panic(err)
		}
		err = instance.RegisterValidation(BGVersion, bgVersionValidation)
		if err != nil {
			panic(err)
		}
	})

	return instance
}

func Validate(data any) error {
	err := Get().Struct(data)
	var ve = new(validator.ValidationErrors)
	if err != nil && errors.As(err, ve) {
		// reformat error
		err = fmt.Errorf("%v: %w", strings.Join(FormatErrors(*ve), "\n "), msg.BadRequest)
	}
	return err
}

// TODO hide from public scope, force to use Validate()
func FormatErrors(ve validator.ValidationErrors) []string {
	var errs []string
	for _, err := range ve {
		fieldPath := err.Namespace()
		if strings.Contains(fieldPath, ".") {
			fieldPath = strings.ToLower(strings.Join(strings.Split(err.Namespace(), ".")[1:], "."))
		}
		var element string
		switch err.Tag() {
		case ExcludedWithTag:
			element = fmt.Sprintf("%[1]s used with %[2]s. %[1]s and %[2]s is mutual exclusive, you can use only one at a time", fieldPath, err.Param())
		case RequiredTag:
			element = fmt.Sprintf("%s is required", fieldPath)
		case ClassifierTag:
			element = fmt.Sprintf("classifier must contain 'name' and 'namespace' fields")
		case AmqpOrAmqps:
			element = fmt.Sprintf("%s must have amqp:// or amqps:// schema", fieldPath)
		case HttpOrHttps:
			element = fmt.Sprintf("%s must have http:// or https:// schema", fieldPath)
		case KafkaProtocol:
			element = fmt.Sprintf("%s must be one of '%s', '%s', '%s' or '%s'", fieldPath,
				model.Plaintext, model.SaslPlaintext, model.Ssl, model.SaslSsl)
		case PlainPassword:
			element = fmt.Sprintf("%s must be '%s'", fieldPath, model.PlainAuth)
		case KafkaAuth:
			element = fmt.Sprintf("%s must be one of '%s', '%s', '%s','%s' or '%s'",
				fieldPath, model.PlainAuth, model.SslCertAuth, model.SCRAMAuth, model.SslCertPlusPlain, model.SslCertPlusSCRAM)
		case KafkaCredentialsRoles:
			element = fmt.Sprintf("%s must have '%s' and '%s' roles", fieldPath, model.Admin, model.Client)
		case "len":
			element = fmt.Sprintf("%s length must be equal to '%s'", err.Namespace(), err.Param())
		case "oneof":
			element = fmt.Sprintf("%s must be one of '%s'", err.Namespace(), err.Param())
		case BGVersion:
			element = fmt.Sprintf("%s must have /v\\d+ pattern", fieldPath)
		case "required_with":
			element = fmt.Sprintf("%s is required if %s is set", fieldPath, err.Param())
		case "eq_ignore_case":
			element = fmt.Sprintf("%s must be equal to '%s'", fieldPath, err.Param())
		case "gt":
			element = fmt.Sprintf("%s must be greater than '%s'", fieldPath, err.Param())
		case "lte":
			element = fmt.Sprintf("%s length must be less than or equal to '%s'", fieldPath, err.Param())
		default:
			element = err.Error()
		}
		errs = append(errs, element)
	}
	return errs
}

func classifierValidation(fl validator.FieldLevel) bool {
	if fl == nil {
		return false
	}
	hasValidClassifierName := false
	hasValidClassifierNamespace := false
	iter := fl.Field().MapRange()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		if k.String() == "name" && fmt.Sprintf("%v", v) != "" {
			hasValidClassifierName = true
		}
		if k.String() == "namespace" && fmt.Sprintf("%v", v) != "" {
			hasValidClassifierNamespace = true
		}
	}

	return hasValidClassifierName && hasValidClassifierNamespace
}

func removeTrailingSlashes(fl validator.FieldLevel) bool {
	if fl != nil {
		fl.Field().SetString(strings.TrimSuffix(fl.Field().String(), "/"))
	}

	return true
}

func kafkaCredentialsRoles(fl validator.FieldLevel) bool {
	if fl != nil {
		v := fl.Field().Interface()
		if creds, ok := v.(map[model.KafkaRole][]model.KafkaCredentials); ok {
			if creds == nil {
				return true
			}
			_, hasAdminRole := creds[model.Admin]
			_, hasClientRole := creds[model.Client]
			if hasAdminRole && hasClientRole {
				return true
			}
		}
	}

	return false
}

func bgVersionValidation(fl validator.FieldLevel) bool {
	if fl != nil {
		return validBgVersion.Match([]byte(fl.Field().String()))
	}
	return true
}

func ValidateClassifier(classifier *model.Classifier) error {
	err := Get().Var(classifier, ClassifierTag)
	if err != nil {
		return fmt.Errorf("invalid classifier %+v: %v: %w", classifier, err.Error(), msg.BadRequest)
	}
	return nil
}
