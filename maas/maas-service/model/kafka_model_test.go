package model

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTopicRegistration_SettingsAreEqual(t *testing.T) {
	type fields struct {
		Classifier        Classifier
		Topic             string
		Instance          string
		Namespace         string
		ExternallyManaged bool
		TopicSettings     TopicSettings
		Template          sql.NullInt64
		Dirty             bool
		TenantId          sql.NullInt64
		InstanceRef       *KafkaInstance
	}
	type args struct {
		anotherTopic *TopicRegistration
	}
	var minNumPartitions int32 = 42
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{name: "nil", fields: fields{
			TopicSettings: TopicSettings{
				MinNumPartitions: nil,
			},
		}, args: struct{ anotherTopic *TopicRegistration }{anotherTopic: &TopicRegistration{
			TopicSettings: TopicSettings{
				MinNumPartitions: nil,
			},
		}}, want: true},
		{name: "first_different", fields: fields{
			TopicSettings: TopicSettings{
				MinNumPartitions: &minNumPartitions,
			},
		}, args: struct{ anotherTopic *TopicRegistration }{anotherTopic: &TopicRegistration{
			TopicSettings: TopicSettings{
				MinNumPartitions: nil,
			},
		}}, want: false},
		{name: "second_different", fields: fields{
			TopicSettings: TopicSettings{
				MinNumPartitions: &minNumPartitions,
			},
		}, args: struct{ anotherTopic *TopicRegistration }{anotherTopic: &TopicRegistration{
			TopicSettings: TopicSettings{
				MinNumPartitions: nil,
			},
		}}, want: false},
		{name: "equals", fields: fields{
			TopicSettings: TopicSettings{
				MinNumPartitions: &minNumPartitions,
			},
		}, args: struct{ anotherTopic *TopicRegistration }{anotherTopic: &TopicRegistration{
			TopicSettings: TopicSettings{
				MinNumPartitions: &minNumPartitions,
			},
		}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := &TopicRegistration{
				Classifier:        &tt.fields.Classifier,
				Topic:             tt.fields.Topic,
				Instance:          tt.fields.Instance,
				Namespace:         tt.fields.Namespace,
				ExternallyManaged: tt.fields.ExternallyManaged,
				TopicSettings:     tt.fields.TopicSettings,
				Template:          tt.fields.Template,
				Dirty:             tt.fields.Dirty,
				TenantId:          tt.fields.TenantId,
				InstanceRef:       tt.fields.InstanceRef,
			}
			assert.Equalf(t, tt.want, topic.SettingsAreEqual(tt.args.anotherTopic), "SettingsAreEqual(%v)", tt.args.anotherTopic)
		})
	}
}

func TestTopicSettings_ActualNumPartitions(t *testing.T) {
	type fields struct {
		NumPartitions     *int32
		MinNumPartitions  *int32
		ReplicationFactor *int16
		ReplicaAssignment map[int32][]int32
		Configs           map[string]*string
	}
	var partNum int32 = 42

	tests := []struct {
		name   string
		fields fields
		want   int32
	}{
		{name: "nil", fields: fields{
			MinNumPartitions: nil,
		}, want: 1},
		{name: "num_partitions", fields: fields{
			NumPartitions:    &partNum,
			MinNumPartitions: nil,
		}, want: partNum},
		{name: "min_num_partitions", fields: fields{
			NumPartitions:    nil,
			MinNumPartitions: &partNum,
		}, want: partNum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &TopicSettings{
				NumPartitions:     tt.fields.NumPartitions,
				MinNumPartitions:  tt.fields.MinNumPartitions,
				ReplicationFactor: tt.fields.ReplicationFactor,
				ReplicaAssignment: tt.fields.ReplicaAssignment,
				Configs:           tt.fields.Configs,
			}
			assert.Equalf(t, tt.want, settings.ActualNumPartitions(), "ActualNumPartitions()")
		})
	}
}

func TestTopicSettings_ReadOnlySettingsAreEqual(t *testing.T) {
	type fields struct {
		NumPartitions     *int32
		MinNumPartitions  *int32
		ReplicationFactor *int16
		ReplicaAssignment map[int32][]int32
		Configs           map[string]*string
	}
	type args struct {
		anotherSettings *TopicSettings
	}
	var partNum int32 = 42

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{name: "nil", fields: fields{
			MinNumPartitions: nil,
		}, args: struct{ anotherSettings *TopicSettings }{anotherSettings: &TopicSettings{MinNumPartitions: nil}}, want: true},
		{name: "first_different", fields: fields{
			MinNumPartitions: &partNum,
		}, args: struct{ anotherSettings *TopicSettings }{anotherSettings: &TopicSettings{MinNumPartitions: nil}}, want: false},
		{name: "second_different", fields: fields{
			MinNumPartitions: nil,
		}, args: struct{ anotherSettings *TopicSettings }{anotherSettings: &TopicSettings{MinNumPartitions: &partNum}}, want: false},
		{name: "equals", fields: fields{
			MinNumPartitions: &partNum,
		}, args: struct{ anotherSettings *TopicSettings }{anotherSettings: &TopicSettings{MinNumPartitions: &partNum}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &TopicSettings{
				NumPartitions:     tt.fields.NumPartitions,
				MinNumPartitions:  tt.fields.MinNumPartitions,
				ReplicationFactor: tt.fields.ReplicationFactor,
				ReplicaAssignment: tt.fields.ReplicaAssignment,
				Configs:           tt.fields.Configs,
			}
			assert.Equalf(t, tt.want, settings.ReadOnlySettingsAreEqual(tt.args.anotherSettings), "ReadOnlySettingsAreEqual(%v)", tt.args.anotherSettings)
		})
	}
}

func TestTopicRegistration_ToResponseDto_PlaintextAlias(t *testing.T) {
	saslPlaintext := []string{"kafka:9092"}
	plaintext := []string{"kafka:9094"}
	saslSsl := []string{"kafka:9093"}
	tests := []struct {
		name      string
		addresses map[KafkaProtocol][]string
		client    []KafkaCredentials
		want      map[KafkaProtocol][]string
	}{
		{
			name:      "plain on SASL_PLAINTEXT gets PLAINTEXT alias",
			addresses: map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext},
			client:    []KafkaCredentials{{AuthType: PlainAuth, Username: "app", Password: "plain:secret"}},
			want:      map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext, Plaintext: saslPlaintext},
		},
		{
			name:      "alias keeps other listeners",
			addresses: map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext, SaslSsl: saslSsl},
			client:    []KafkaCredentials{{AuthType: PlainAuth}},
			want:      map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext, SaslSsl: saslSsl, Plaintext: saslPlaintext},
		},
		{
			name:      "real PLAINTEXT listener is not overwritten",
			addresses: map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext, Plaintext: plaintext},
			client:    []KafkaCredentials{{AuthType: PlainAuth}},
			want:      map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext, Plaintext: plaintext},
		},
		{
			name:      "SCRAM needs no alias",
			addresses: map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext},
			client:    []KafkaCredentials{{AuthType: SCRAMAuth}},
			want:      map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext},
		},
		{
			name:      "no client credentials",
			addresses: map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext},
			want:      map[KafkaProtocol][]string{SaslPlaintext: saslPlaintext},
		},
		{
			name:      "no SASL_PLAINTEXT listener",
			addresses: map[KafkaProtocol][]string{SaslSsl: saslSsl},
			client:    []KafkaCredentials{{AuthType: PlainAuth}},
			want:      map[KafkaProtocol][]string{SaslSsl: saslSsl},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := map[KafkaProtocol][]string{}
			for protocol, servers := range tt.addresses {
				stored[protocol] = servers
			}
			instance := &KafkaInstance{Addresses: tt.addresses, Credentials: map[KafkaRole][]KafkaCredentials{Admin: tt.client, Client: tt.client}}
			topic := &TopicRegistration{Classifier: &Classifier{Name: "orders", Namespace: "orders"}, Topic: "orders", InstanceRef: instance}

			assert.Equal(t, tt.want, topic.ToResponseDto().Addresses)
			assert.Equal(t, stored, instance.Addresses, "instance addresses must not change")
		})
	}
}
