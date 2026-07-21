package helper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/IBM/sarama"
	"github.com/netcracker/qubership-maas/model"
)

const (
	certFormatSeparator = "-----"
	certificate         = "CERTIFICATE"
	privateKey          = "PRIVATE KEY"
	certFormat          = certFormatSeparator + "BEGIN %s" + certFormatSeparator + "\n%s\n" + certFormatSeparator + "END %s" + certFormatSeparator
)

var ErrNoCACert = errors.New("kafka: CA certificate must be configured for SSL connection to kafka")

func (helper *HelperImpl) createClusterAdmin(ctx context.Context, instance *model.KafkaInstance) (sarama.ClusterAdmin, error) {
	return createConnection(helper, ctx, instance, helper.client.NewClusterAdmin)
}

// createClient opens a low level sarama client. It is used when only cluster metadata is needed
// (e.g. the list of topic names via Client.Topics()) without the extra DescribeConfigs round trip
// that ClusterAdmin.ListTopics() performs for every topic.
func (helper *HelperImpl) createClient(ctx context.Context, instance *model.KafkaInstance) (sarama.Client, error) {
	return createConnection(helper, ctx, instance, helper.client.NewClient)
}

// createConnection builds the sarama config for the instance and opens a connection with the given
// factory (NewClusterAdmin or NewClient).
func createConnection[T any](helper *HelperImpl, ctx context.Context, instance *model.KafkaInstance, factory func([]string, *sarama.Config) (T, error)) (T, error) {
	var zero T
	config, err := helper.buildConfig(ctx, instance)
	if err != nil {
		return zero, err
	}
	addresses := instance.Addresses[instance.MaasProtocol]
	conn, err := factory(addresses, config)
	if err != nil {
		log.ErrorC(ctx, "Failed to create kafka %s for %+v: %v", reflect.TypeFor[T](), addresses, err)
		return zero, err
	}
	return conn, nil
}

func (helper *HelperImpl) buildConfig(ctx context.Context, instance *model.KafkaInstance) (*sarama.Config, error) {
	config := sarama.NewConfig()

	config.Admin.Timeout = helper.KafkaClientTimeout
	config.ClientID = "maas"
	config.Version = sarama.V2_8_0_0

	useTls := instance.MaasProtocol == model.Ssl || instance.MaasProtocol == model.SaslSsl
	if useTls {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = &tls.Config{}
		if instance.CACert != "" {
			// user specifies custom cert
			config.Net.TLS.Config.RootCAs = formatCert(instance.CACert)
		}
	}

	if credentialsList, found := instance.Credentials[model.Admin]; found && len(credentialsList) > 0 {
		credentials := credentialsList[0]
		authType := credentials.GetAuthType()
		switch authType {
		case model.SslCertAuth:
			if !useTls {
				log.DebugC(ctx, "Admin SSL authorization is skipped due to using PLAINTEXT protocol")
			} else if err := fillSslClientCert(ctx, config, credentials); err != nil {
				return nil, err
			}
		case model.PlainAuth:
			if err := fillPlainSaslAuth(ctx, config, credentials); err != nil {
				return nil, err
			}
		case model.SslCertPlusPlain:
			if useTls {
				if err := fillSslClientCert(ctx, config, credentials); err != nil {
					return nil, err
				}
			} else {
				log.DebugC(ctx, "Admin SSL authorization is skipped due to using PLAINTEXT protocol")
			}
			if err := fillPlainSaslAuth(ctx, config, credentials); err != nil {
				return nil, err
			}
		case model.SCRAMAuth:
			if err := fillSaslSCRAMAuth(ctx, config, credentials); err != nil {
				return nil, err
			}
		case model.SslCertPlusSCRAM:
			if useTls {
				if err := fillSslClientCert(ctx, config, credentials); err != nil {
					return nil, err
				}
			} else {
				log.DebugC(ctx, "Admin SSL authorization is skipped due to using PLAINTEXT protocol")
			}
			if err := fillSaslSCRAMAuth(ctx, config, credentials); err != nil {
				return nil, err
			}
		default:
			errMsg := fmt.Sprintf("kafka: %s auth is not supported by this MaaS release", credentials.GetAuthType())
			return nil, errors.New(errMsg)
		}
	}

	return config, nil
}

func fillSslClientCert(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials) error {
	auth := credentials.GetSslCertAuth()
	// Load client cert
	clientCert := formatCertificate(auth.ClientCert)
	clientKey := formatPrivateKey(auth.ClientKey)
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		log.ErrorC(ctx, "Admin SSL authorization has failed: %v", err)
		return err
	}
	config.Net.TLS.Config.Certificates = []tls.Certificate{cert}
	return nil
}

func fillPlainSaslAuth(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials) error {
	auth := credentials.GetBasicAuth()
	pass, err := resolvePassword(auth.Password)
	if err != nil {
		log.ErrorC(ctx, "Failed to resolve admin password for kafka instance: %v", err)
		return err
	}
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	config.Net.SASL.User = auth.Username
	config.Net.SASL.Password = pass
	return nil
}

func fillSaslSCRAMAuth(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials) error {
	auth := credentials.GetBasicAuth()
	pass, err := resolvePassword(auth.Password)
	if err != nil {
		log.ErrorC(ctx, "Failed to resolve admin password for kafka instance: %v", err)
		return err
	}
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	config.Net.SASL.User = auth.Username
	config.Net.SASL.Password = pass
	config.Net.SASL.SCRAMClientGeneratorFunc = GenerateSCRAMSHA512Client
	return nil
}

func resolvePassword(passwordWithPrefix []byte) (string, error) {
	typeAndPass := strings.SplitN(string(passwordWithPrefix), ":", 2)
	passType := model.PasswordType(typeAndPass[0])
	switch passType {
	case model.Plain:
		return typeAndPass[1], nil
	default:
		return "", fmt.Errorf("kafka: password type %s is not supported", passType)
	}
}

func formatPem(originalPem, certType string) []byte {
	if strings.Contains(originalPem, certFormatSeparator) {
		return []byte(originalPem)
	}
	return []byte(fmt.Sprintf(certFormat, certType, originalPem, certType))
}

func formatCertificate(originalPem string) []byte {
	return formatPem(originalPem, certificate)
}

func formatPrivateKey(originalPem string) []byte {
	return formatPem(originalPem, privateKey)
}

func formatCert(caCertStr string) *x509.CertPool {
	// Load CA cert
	caCert := formatCertificate(caCertStr)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return caCertPool
}
