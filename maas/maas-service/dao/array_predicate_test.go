package dao

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run against a real connection: what matters is which rows come back, not the SQL text.
func TestArrayContains_OnSqliteReadsTheArrayLiteral(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"))
	require.NoError(t, err)
	require.NoError(t, db.Exec("create table templates (name text, domain_namespaces ARRAY)").Error)
	require.NoError(t, db.Exec(`insert into templates values
		('first', '{alpha,beta}'),
		('single', '{gamma}'),
		('empty', '{}'),
		('none', null)`).Error)

	found := func(namespace string) []string {
		predicate, args := ArrayContains(db, "domain_namespaces", namespace)
		var names []string
		require.NoError(t, db.Table("templates").Where(predicate, args...).Order("name").Pluck("name", &names).Error)
		return names
	}

	assert.Equal(t, []string{"first"}, found("alpha"), "first element of a multi-element array")
	assert.Equal(t, []string{"first"}, found("beta"), "last element of a multi-element array")
	assert.Equal(t, []string{"single"}, found("gamma"), "the only element")
	assert.Empty(t, found("delta"), "a namespace no row carries")
	assert.Empty(t, found("alph"), "a prefix of an element is not the element")
	assert.Empty(t, found("lpha"), "a suffix of an element is not the element")
}

func TestArrayContains_OnPostgresUsesTheArrayOperator(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{Dialector: postgres.New(postgres.Config{})}}

	predicate, args := ArrayContains(db, "domain_namespaces", "alpha")

	assert.Equal(t, "?=ANY(domain_namespaces)", predicate)
	assert.Equal(t, []any{"alpha"}, args)
}
