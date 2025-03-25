package helper

import (
	"crypto/sha256"
	"crypto/sha512"
	"github.com/IBM/sarama"
	"github.com/xdg/scram"
	"hash"
)

var SHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
var SHA512 scram.HashGeneratorFcn = func() hash.Hash { return sha512.New() }

func GenerateSCRAMSHA512Client() sarama.SCRAMClient {
	return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
}

func GenerateSCRAMSHA256Client() sarama.SCRAMClient {
	return &XDGSCRAMClient{HashGeneratorFcn: SHA256}
}

// XDGSCRAMClient implementation is taken from sample github.com/!shopify/sarama@v1.26.4/examples/sasl_scram_client/scram_client.go
type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
