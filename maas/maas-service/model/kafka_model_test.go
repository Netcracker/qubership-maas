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
