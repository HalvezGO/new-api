package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestShouldWriteInProgressLog(t *testing.T) {
	originalLogConsume := common.LogConsumeEnabled
	originalLogType := common.LogDatabaseType()
	t.Cleanup(func() {
		common.LogConsumeEnabled = originalLogConsume
		common.SetLogDatabaseType(originalLogType)
	})

	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.LogConsumeEnabled = true
	assert.True(t, ShouldWriteInProgressLog(), "enabled on a supported log database")

	common.LogConsumeEnabled = false
	assert.False(t, ShouldWriteInProgressLog(), "disabled when consume logging is off")

	common.LogConsumeEnabled = true
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	assert.False(t, ShouldWriteInProgressLog(), "disabled on ClickHouse log database")
}
