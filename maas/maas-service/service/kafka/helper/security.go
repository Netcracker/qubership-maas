package helper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
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
	config, addresses, err := helper.buildSaramaConfig(ctx, instance, model.Admin)
	if err != nil {
		return nil, err
	}
	admin, err := helper.client.NewClusterAdmin(addresses, config)
	if err != nil {
		log.ErrorC(ctx, "Failed to create kafka admin client for %+v: %v", addresses, err)
		return nil, err
	}
	return admin, nil
}

func (helper *HelperImpl) createClient(ctx context.Context, instance *model.KafkaInstance) (io.Closer, error) {
	config, addresses, err := helper.buildSaramaConfig(ctx, instance, model.Client)
	if err != nil {
		return nil, err
	}
	client, err := helper.client.NewClient(addresses, config)
	if err != nil {
		log.ErrorC(ctx, "Failed to create kafka client for %+v: %v", addresses, err)
		return nil, err
	}
	return client, nil
}

func (helper *HelperImpl) buildSaramaConfig(ctx context.Context, instance *model.KafkaInstance, role model.KafkaRole) (*sarama.Config, []string, error) {
	config := sarama.NewConfig()

	config.Admin.Timeout = helper.KafkaClientTimeout
	config.Metadata.Timeout = helper.KafkaClientTimeout
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

	if credentialsList, found := instance.Credentials[role]; found && len(credentialsList) > 0 {
		if err := fillAuth(ctx, config, credentialsList[0], useTls, role); err != nil {
			return nil, nil, err
		}
	}

	return config, instance.Addresses[instance.MaasProtocol], nil
}

func fillAuth(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials, useTls bool, role model.KafkaRole) error {
	authType := credentials.GetAuthType()
	switch authType {
	case model.SslCertAuth:
		if !useTls {
			log.DebugC(ctx, "%s SSL authorization is skipped due to using PLAINTEXT protocol", role)
		} else if err := fillSslClientCert(ctx, config, credentials); err != nil {
			return err
		}
	case model.PlainAuth:
		if err := fillPlainSaslAuth(ctx, config, credentials); err != nil {
			return err
		}
	case model.SslCertPlusPlain:
		if useTls {
			if err := fillSslClientCert(ctx, config, credentials); err != nil {
				return err
			}
		} else {
			log.DebugC(ctx, "%s SSL authorization is skipped due to using PLAINTEXT protocol", role)
		}
		if err := fillPlainSaslAuth(ctx, config, credentials); err != nil {
			return err
		}
	case model.SCRAMAuth:
		if err := fillSaslSCRAMAuth(ctx, config, credentials); err != nil {
			return err
		}
	case model.SslCertPlusSCRAM:
		if useTls {
			if err := fillSslClientCert(ctx, config, credentials); err != nil {
				return err
			}
		} else {
			log.DebugC(ctx, "%s SSL authorization is skipped due to using PLAINTEXT protocol", role)
		}
		if err := fillSaslSCRAMAuth(ctx, config, credentials); err != nil {
			return err
		}
	default:
		return fmt.Errorf("kafka: %s auth is not supported by this MaaS release", credentials.GetAuthType())
	}
	return nil
}

func fillSslClientCert(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials) error {
	auth := credentials.GetSslCertAuth()
	// Load client cert
	clientCert := formatCertificate(auth.ClientCert)
	clientKey := formatPrivateKey(auth.ClientKey)
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		log.ErrorC(ctx, "SSL authorization has failed: %v", err)
		return err
	}
	config.Net.TLS.Config.Certificates = []tls.Certificate{cert}
	return nil
}

func fillPlainSaslAuth(ctx context.Context, config *sarama.Config, credentials model.KafkaCredentials) error {
	auth := credentials.GetBasicAuth()
	pass, err := resolvePassword(auth.Password)
	if err != nil {
		log.ErrorC(ctx, "Failed to resolve password for kafka instance: %v", err)
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
		log.ErrorC(ctx, "Failed to resolve password for kafka instance: %v", err)
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
