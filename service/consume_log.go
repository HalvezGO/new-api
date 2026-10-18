package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// ShouldWriteInProgressLog reports whether requests should write an in-progress
// consume log row. ClickHouse log databases are excluded: their append-only
// model turns each in-place finalize into a heavy table mutation.
func ShouldWriteInProgressLog() bool {
	if !common.LogConsumeEnabled || !common.InProgressLogEnabled {
		return false
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return false
	}
	return true
}

// RecordPendingConsumeLog writes the in-progress placeholder row for a request
// and stores its id on relayInfo. Returns 0 (and writes nothing) when the
// feature is off, the log database is ClickHouse, or the insert failed.
func RecordPendingConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo) int {
	if !ShouldWriteInProgressLog() || relayInfo == nil {
		return 0
	}
	// Realtime sessions are long-lived WebSockets with no natural request end and
	// can outlive any sweep threshold, so they keep today's settle-time logging.
	if relayInfo.RelayFormat == types.RelayFormatOpenAIRealtime {
		return 0
	}
	id, err := model.RecordPendingConsumeLog(c, relayInfo.UserId, model.RecordConsumeLogParams{
		ModelName: relayInfo.OriginModelName,
		TokenName: c.GetString("token_name"),
		TokenId:   relayInfo.TokenId,
		Group:     relayInfo.UsingGroup,
		IsStream:  relayInfo.IsStream,
		ChannelId: relayInfo.ChannelId,
		Other: map[string]interface{}{
			"in_progress": true,
		},
	})
	if err != nil {
		return 0
	}
	relayInfo.ConsumeLogId = id
	return id
}

// finalizeConsumeLog finalizes the request's pending row with settled values,
// falling back to a plain insert when no pending row exists (feature off,
// insert failed, ClickHouse) or the row was already finalized/swept.
func finalizeConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, userId int, params model.RecordConsumeLogParams) {
	if relayInfo == nil {
		model.RecordConsumeLog(c, userId, params)
		return
	}
	mergeRetryFailures(params.Other, relayInfo)
	if relayInfo.ConsumeLogId <= 0 || relayInfo.ConsumeLogFinalized {
		model.RecordConsumeLog(c, userId, params)
		return
	}
	relayInfo.ConsumeLogFinalized = true
	updated, err := model.UpdateConsumeLogFinal(c, relayInfo.ConsumeLogId, params)
	if err == nil && updated {
		return
	}
	if err != nil {
		common.SysLog("finalize pending consume log failed, inserted completed row instead: " + err.Error())
	} else {
		common.SysLog(fmt.Sprintf(
			"pending consume log row %d was already finalized or swept, inserted completed row instead",
			relayInfo.ConsumeLogId,
		))
	}
	model.RecordConsumeLog(c, userId, params)
}

// MarkConsumeLogFailed finalizes the request's pending row as failed. Called
// from Relay()'s deferred error handler; a no-op when no pending row exists or
// the row was already finalized on the success path.
func MarkConsumeLogFailed(c *gin.Context, relayInfo *relaycommon.RelayInfo, err *types.NewAPIError) {
	if relayInfo == nil || relayInfo.ConsumeLogId <= 0 || relayInfo.ConsumeLogFinalized {
		return
	}
	relayInfo.ConsumeLogFinalized = true

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	useTimeSeconds := 0
	if !startTime.IsZero() {
		useTimeSeconds = int(time.Since(startTime).Seconds())
	}
	other := map[string]interface{}{
		"in_progress": false,
	}
	mergeRetryFailures(other, relayInfo)
	if len(relayInfo.ConsumeLogRetryFailures) > 0 {
		last := relayInfo.ConsumeLogRetryFailures[len(relayInfo.ConsumeLogRetryFailures)-1]
		other["failure_reason"] = last.Reason
		other["admin_info"] = map[string]interface{}{
			"retry_attempts": last.RetryIndex + 1,
		}
	} else if err != nil {
		other["failure_reason"] = common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())
	}

	if _, updateErr := model.UpdateConsumeLogFailed(c, relayInfo.ConsumeLogId, useTimeSeconds, relayInfo.ChannelId, other); updateErr != nil {
		common.SysLog("mark pending consume log failed: " + updateErr.Error())
	}
}

// AppendRetryFailure records one failed channel attempt on the request's
// consume-log row. Called in place of RecordErrorLog when a pending row exists,
// so N retries never produce N extra log rows.
func AppendRetryFailure(relayInfo *relaycommon.RelayInfo, channelError types.ChannelError, err *types.NewAPIError) {
	if relayInfo == nil {
		return
	}
	if len(relayInfo.ConsumeLogRetryFailures) >= relaycommon.MaxRetryFailureRecords {
		relayInfo.ConsumeLogRetryFailuresTruncated = true
		return
	}
	record := relaycommon.RetryFailureRecord{
		RetryIndex:   relayInfo.RetryIndex,
		ChannelId:    channelError.ChannelId,
		ChannelName:  channelError.ChannelName,
		WaitedMs:     relayInfo.LastRetryWaitMs,
	}
	if err != nil {
		record.StatusCode = err.StatusCode
		record.ErrorCode = string(err.GetErrorCode())
		record.Reason = common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())
	}
	relayInfo.ConsumeLogRetryFailures = append(relayInfo.ConsumeLogRetryFailures, record)
}

// mergeRetryFailures folds accumulated per-attempt failure summaries into a log
// row's other map. Failed attempts belong to the request, so they ride on the
// single row instead of one error-log row each.
func mergeRetryFailures(other map[string]interface{}, relayInfo *relaycommon.RelayInfo) {
	if other == nil || relayInfo == nil || len(relayInfo.ConsumeLogRetryFailures) == 0 {
		return
	}
	other["retry_failures"] = relayInfo.ConsumeLogRetryFailures
	if relayInfo.ConsumeLogRetryFailuresTruncated {
		other["retry_failures_truncated"] = true
	}
	logger.LogDebug(nil, fmt.Sprintf("merged %d retry failures into consume log", len(relayInfo.ConsumeLogRetryFailures)))
}
