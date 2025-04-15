package postdeploy

import (
	"context"
	"github.com/netcracker/qubership-maas/model"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestBrokerRegistration_SinlgeInstanceLoad(t *testing.T) {
	instanceJson := `{"id": "default", "addresses": { "PLAINTEXT": ["kafka.kafka-argo:9092"] }, "maasProtocol": "PLAINTEXT"}`
	withTempFile(t, instanceJson, func(filename string) {
		err := readInstance(context.Background(), filename, func(objects []model.KafkaInstance) error {
			assert.NotNil(t, objects)
			return nil
		})
		assert.NoError(t, err)
	})
}

func TestBrokerRegistration_ArrayInstanceLoad(t *testing.T) {
	instanceJson := `[{"id": "default", "addresses": { "PLAINTEXT": ["kafka.kafka-argo:9092"] }, "maasProtocol": "PLAINTEXT"}]`
	withTempFile(t, instanceJson, func(filename string) {
		err := readInstance(context.Background(), filename, func(objects []model.KafkaInstance) error {
			assert.Equal(t, 1, len(objects))
			return nil
		})
		assert.NoError(t, err)
	})
}

func withTempFile(t *testing.T, content string, callback func(path string)) {
	accountsDir, err := os.CreateTemp(os.TempDir(), "maas-test-file*")
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(accountsDir, content)
	if err != nil {
		panic(err)
	}
	defer os.Remove(accountsDir.Name())

	callback(accountsDir.Name())
}
