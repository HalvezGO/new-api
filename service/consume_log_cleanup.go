package service

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	inProgressLogCleanupInterval = time.Hour
	// Recovery updates chunk their ids: SQLite caps the bound parameters of a
	// single statement, and orphaned rows can pile up while a node is down.
	inProgressLogRecoveryBatchSize = 200
)

// StartInProgressLogCleanup marks in-progress consume log rows that can no longer
// finalize as timed out: rows left behind by a previous process are recovered
// once at startup on every node, and rows older than the stale threshold are
// swept by the master. Mirrors StartAuthArtifactCleanup.
func StartInProgressLogCleanup() {
	go func() {
		recoverOrphanedInProgressLogs()
		if common.IsMasterNode {
			sweepStaleInProgressLogs()
		}
		// A node that died moments before this process started still looks alive
		// until its heartbeat goes stale, so run one more recovery pass after that
		// window before settling into the hourly sweep.
		time.Sleep(time.Duration(model.SystemInstanceStaleAfterSeconds+30) * time.Second)
		recoverOrphanedInProgressLogs()
		if !common.IsMasterNode {
			return
		}
		ticker := time.NewTicker(inProgressLogCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			sweepStaleInProgressLogs()
		}
	}()
}

func sweepStaleInProgressLogs() {
	// ClickHouse has no UPDATE statement; the feature is disabled there anyway.
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return
	}
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

// recoverOrphanedInProgressLogs runs once per process: a pending row created
// before this process started cannot belong to an in-flight request of ours, so
// when it was left by this node's previous process, by a node that no longer
// reports heartbeats, or (on a single-node deployment) by a version that did not
// record the serving node, it is marked as timed out instead of lingering as
// "in progress" until the stale sweep.
func recoverOrphanedInProgressLogs() {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return
	}
	logs, err := model.ListInProgressConsumeLogsBefore(common.StartTime)
	if err != nil {
		common.SysError("failed to list orphaned in-progress consume logs: " + err.Error())
		return
	}
	if len(logs) == 0 {
		return
	}
	otherLiveNodes, err := liveSystemInstanceNames()
	if err != nil {
		common.SysError("failed to read live instances for in-progress log recovery: " + err.Error())
		return
	}
	ids := orphanedInProgressLogIDs(logs, common.NodeName, otherLiveNodes)
	var swept int64
	for start := 0; start < len(ids); start += inProgressLogRecoveryBatchSize {
		end := start + inProgressLogRecoveryBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		updated, updateErr := model.MarkInProgressConsumeLogsTimedOut(ids[start:end])
		if updateErr != nil {
			common.SysError("failed to recover orphaned in-progress consume logs: " + updateErr.Error())
			return
		}
		swept += updated
	}
	if swept > 0 {
		common.SysLog("recovered orphaned in-progress consume logs: " + strconv.FormatInt(swept, 10))
	}
}

// orphanedInProgressLogIDs selects the pending rows that cannot still be
// running: rows owned by this node (the process just started, so they belong to
// a previous incarnation), rows whose owner is not live, and legacy rows without
// an owner when no other node is serving.
func orphanedInProgressLogIDs(logs []model.Log, currentNode string, otherLiveNodes map[string]bool) []int {
	var ids []int
	for _, entry := range logs {
		node := pendingConsumeLogNodeName(entry.Other)
		switch {
		case currentNode != "" && node == currentNode:
			ids = append(ids, entry.Id)
		case node == "":
			if len(otherLiveNodes) == 0 {
				ids = append(ids, entry.Id)
			}
		case !otherLiveNodes[node]:
			ids = append(ids, entry.Id)
		}
	}
	return ids
}

// liveSystemInstanceNames returns the nodes other than this one whose heartbeat
// is still fresh.
func liveSystemInstanceNames() (map[string]bool, error) {
	instances, err := model.ListSystemInstances()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	live := make(map[string]bool, len(instances))
	for _, instance := range instances {
		if instance == nil || instance.NodeName == "" || instance.NodeName == common.NodeName {
			continue
		}
		if now-instance.LastSeenAt > model.SystemInstanceStaleAfterSeconds {
			continue
		}
		live[instance.NodeName] = true
	}
	return live, nil
}

func pendingConsumeLogNodeName(other string) string {
	if other == "" {
		return ""
	}
	var payload struct {
		AdminInfo struct {
			NodeName string `json:"node_name"`
		} `json:"admin_info"`
	}
	if err := common.UnmarshalJsonStr(other, &payload); err != nil {
		return ""
	}
	return payload.AdminInfo.NodeName
}
