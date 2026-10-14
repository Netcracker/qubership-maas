package model

// InstanceOwnership records which KafkaInstance / RabbitInstance CR owns a broker instance row.
// Not part of the manager REST API: REST bodies can neither read nor set it.
type InstanceOwnership struct {
	ManagedByOperator bool   `gorm:"column:managed_by_operator" pg:"managed_by_operator,use_zero" json:"-"`
	CRNamespace       string `gorm:"column:cr_namespace" pg:"cr_namespace" json:"-"`
	CRName            string `gorm:"column:cr_name" pg:"cr_name" json:"-"`
}
