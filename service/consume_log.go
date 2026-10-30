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
	"github.com/bytedance/gopkg/util/gopool"
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
	other := model.NewLogOther()
	other.SetPublic("in_progress", true)
	if c != nil && c.Request != nil && c.Request.URL != nil {
		other.SetPublic("request_path", c.Request.URL.Path)
	}
	if common.NodeName != "" {
		// Record the serving node so a restart can tell rows left behind by a
		// dead process from requests still running elsewhere. Admin-only, like the
		// rest of admin_info (stripped from user-facing log payloads).
		other.SetAdmin("node_name", common.NodeName)
	}
	id, err := model.RecordPendingConsumeLog(c, relayInfo.UserId, model.RecordConsumeLogParams{
		ModelName: relayInfo.OriginModelName,
		TokenName: c.GetString("token_name"),
		TokenId:   relayInfo.TokenId,
		Group:     relayInfo.UsingGroup,
		IsStream:  relayInfo.IsStream,
		// No channel is selected yet (ChannelMeta is nil until InitChannelMeta
		// runs in the handler), so use the nil-safe accessor: 0 means "settled
		// later", and the finalize update writes the real channel id.
		ChannelId: relayInfo.GetChannelID(),
		Other:     other,
	})
	if err != nil {
		return 0
	}
	relayInfo.ConsumeLogId = id
	if relayInfo.IsStream && id > 0 {
		// Surface the first-token latency on the still-pending row so the logs
		// list shows progress before the stream ends. One async write per stream;
		// the row is settled with the exact value anyway, and the status guard in
		// the model update makes a late write a no-op instead of clobbering it.
		relayInfo.OnFirstResponse = func() {
			frtMs := float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
			if frtMs <= 0 {
				return
			}
			other.SetPublic("frt", frtMs)
			gopool.Go(func() {
				if _, updateErr := model.UpdateInProgressConsumeLogOther(id, other); updateErr != nil {
					common.SysLog("update in-progress consume log first-token time failed: " + updateErr.Error())
				}
			})
		}
	}
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
	params.CreatedAt = relayInfo.StartTime.Unix()
	relayInfo.ConsumeLogFinalized = true
	updated, err := model.UpdateConsumeLogFinal(c, userId, relayInfo.ConsumeLogId, params)
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

// buildFailedLogOther assembles the other payload for a failed pending row:
// request path, the per-attempt retry history, the last failure reason, and the
// channel chain (admin-only) so the row stays as informative as the error logs
// it replaced.
func buildFailedLogOther(c *gin.Context, relayInfo *relaycommon.RelayInfo, err *types.NewAPIError) *model.LogOther {
	other := model.NewLogOther()
	other.SetPublic("in_progress", false)
	if c != nil && c.Request != nil && c.Request.URL != nil && c.Request.URL.Path != "" {
		other.SetPublic("request_path", c.Request.URL.Path)
	}
	mergeRetryFailures(other, relayInfo)
	if relayInfo.HasSendResponse() {
		// The stream had already produced its first token before failing; keep
		// that latency so the failed row stays comparable with completed rows.
		other.SetPublic("frt", float64(relayInfo.FirstResponseTime.UnixMilli()-relayInfo.StartTime.UnixMilli()))
	}
	if c != nil {
		if useChannel := c.GetStringSlice("use_channel"); len(useChannel) > 0 {
			other.SetAdmin("use_channel", useChannel)
			other.SetAdmin("retry_attempts", len(useChannel))
		}
	}
	if err != nil && types.IsClientGone(err) {
		// The caller hung up: report that instead of the last upstream failure,
		// so the row is not read as a provider problem.
		other.SetPublic("client_gone", true)
		other.SetPublic("failure_reason", common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode()))
		return other
	}
	if len(relayInfo.ConsumeLogRetryFailures) > 0 {
		last := relayInfo.ConsumeLogRetryFailures[len(relayInfo.ConsumeLogRetryFailures)-1]
		other.SetPublic("failure_reason", last.Reason)
		return other
	}
	if err != nil {
		other.SetPublic("failure_reason", common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode()))
	}
	return other
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
	// A failure can land before any channel is selected, so ChannelMeta may
	// still be nil here; the accessor returns 0 instead of panicking.
	if _, updateErr := model.UpdateConsumeLogFailed(c, relayInfo.ConsumeLogId, useTimeSeconds, relayInfo.GetChannelID(), buildFailedLogOther(c, relayInfo, err)); updateErr != nil {
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
		RetryIndex:  relayInfo.RetryIndex,
		ChannelId:   channelError.ChannelId,
		ChannelName: channelError.ChannelName,
		WaitedMs:    relayInfo.LastRetryWaitMs,
	}
	if err != nil {
		record.StatusCode = err.StatusCode
		record.ErrorCode = string(err.GetErrorCode())
		record.Reason = common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode())
	}
	relayInfo.ConsumeLogRetryFailures = append(relayInfo.ConsumeLogRetryFailures, record)
}

// mergeRetryFailures folds accumulated per-attempt failure summaries into a log
// row's other payload. Failed attempts belong to the request, so they ride on
// the single row instead of one error-log row each. It also stamps the retry
// attempt count so the logs list can show a "retried N" marker on the row.
func mergeRetryFailures(other *model.LogOther, relayInfo *relaycommon.RelayInfo) {
	if other == nil || relayInfo == nil || len(relayInfo.ConsumeLogRetryFailures) == 0 {
		return
	}
	other.SetPublic("retry_failures", relayInfo.ConsumeLogRetryFailures)
	if relayInfo.ConsumeLogRetryFailuresTruncated {
		other.SetPublic("retry_failures_truncated", true)
	}
	attempts := relayInfo.ConsumeLogRetryFailures[len(relayInfo.ConsumeLogRetryFailures)-1].RetryIndex + 1
	if attempts > 1 {
		other.SetAdmin("retry_attempts", attempts)
	}
	logger.LogDebug(nil, "merged %d retry failures into consume log", len(relayInfo.ConsumeLogRetryFailures))
}
