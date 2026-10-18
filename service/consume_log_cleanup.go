package service

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const inProgressLogCleanupInterval = time.Hour

// StartInProgressLogCleanup marks in-progress consume log rows that never
// finalized (serving node died mid-request) as timed out. Only the master node
// performs cleanup. Mirrors StartAuthArtifactCleanup.
func StartInProgressLogCleanup() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		sweepStaleInProgressLogs()
		ticker := time.NewTicker(inProgressLogCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			sweepStaleInProgressLogs()
		}
	}()
}

func sweepStaleInProgressLogs() {
	before := time.Now().Add(-time.Duration(common.InProgressLogSweepAfterSeconds) * time.Second).Unix()
	swept, err := model.SweepStaleInProgressLogs(before)
	if err != nil {
		common.SysError("failed to sweep stale in-progress consume logs: " + err.Error())
		return
	}
	if swept > 0 {
		common.SysLog("swept stale in-progress consume logs: " + strconv.FormatInt(swept, 10))
	}
}
