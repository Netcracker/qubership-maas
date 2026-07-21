package helper

import (
	"github.com/IBM/sarama"
)

//go:generate mockgen -destination=mock/admin.go -package=mock_helper github.com/IBM/sarama ClusterAdmin
//go:generate mockgen -destination=mock/client.go -source=sarama.go

// SaramaClient is an interface that allows to use sarama static functions in non-static context,
// which allows DI and mocking for unit tests.
type SaramaClient interface {
	NewClusterAdmin(addrs []string, conf *sarama.Config) (sarama.ClusterAdmin, error)
	NewClient(addrs []string, conf *sarama.Config) (sarama.Client, error)
}

type SaramaClientImpl struct{}

func (client *SaramaClientImpl) NewClusterAdmin(addrs []string, conf *sarama.Config) (sarama.ClusterAdmin, error) {
	return sarama.NewClusterAdmin(addrs, conf)
}

func (client *SaramaClientImpl) NewClient(addrs []string, conf *sarama.Config) (sarama.Client, error) {
	return sarama.NewClient(addrs, conf)
}
