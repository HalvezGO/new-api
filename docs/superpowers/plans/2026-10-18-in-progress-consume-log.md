# 进行中消耗日志 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 请求进入 new-api 即落库一条"请求中"占位消耗日志，结束时同一条精准更新；一次请求（含任意次重试）始终只有一条日志，失败尝试收敛进该行的 `other.retry_failures`。

**Architecture:** logs 表新增 `status` 列（0=已完成/1=请求中/2=失败/3=超时清理）；`controller.Relay()` 在价格计算成功后插入占位行并记录 id 到 `RelayInfo`，三个结算函数与失败 defer 按 id 原地更新；错误日志在存在占位行时改为累积不落库；master 定时清理超时未终值化的行。

**Tech Stack:** Go(Gin/GORM) + React(Rsbuild/i18next/vitest)；SQLite/MySQL/PostgreSQL 日志库（ClickHouse 上功能禁用）。

**Spec:** docs/superpowers/specs/2026-10-18-in-progress-consume-log-design.md

## Global Constraints

- 数据库兼容：SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6；SQLite 迁移照 `model/main.go` 既有 `PRAGMA table_info` + `ALTER TABLE ADD COLUMN` 模式；禁 GORM 布尔默认 tag（本功能 status 用 int 显式赋值）。
- ClickHouse 日志库上功能禁用（`common.UsingLogDatabase(common.DatabaseTypeClickHouse)` 为 true 时不写占位行），但 `status` 列必须加到 CH DDL 与存量迁移（见 Spec §12）。
- 写放大保护：`common.InProgressLogEnabled` 默认 true、`common.LogConsumeEnabled` 必须同时为 true；任一为 false 时零占位行、行为与今天一致。
- 所有日志写失败只 `common.SysError`，绝不影响请求主流程或计费；`retry_failures` 脱敏口径与现有错误日志一致（`common.LocalLogPreview` + `MaskSensitiveErrorWithStatusCode`）。
- 后端新测试用 `github.com/stretchr/testify`（require/assert）；前端新测试放对应模块 `__tests__/` 目录；每任务完成跑受影响测试 + typecheck/lint/format。
- 提交信息用仓库现有 conventional 风格，每个任务一个 commit。

## Review Focus

- **开关关闭/即时切换后的孤儿行**：请求进行中把 `InProgressLogEnabled` 关掉，已插入的占位行在结算时仍按 id 终值化（不丢账、不双写），插入路径与终值化路径互不依赖。
- **一次请求重复终值化**（成功路径结算 + defer 失败路径同时触发）：`WHERE status=1` 守卫 + `relayInfo.ConsumeLogFinalized` 双重保证，第二次调用必须是不产生新行、不改已完成行。
- **`status` 参数缺省值**：controller 层 `status` query 为空时必须解释为 -1（不过滤），而不是 0（只显示已完成），否则"全部"筛选下所有请求中行消失。
- **legacy/CH 查询不炸**：`Log.Status` 字段必须默认 0；CH 无 `status` 列时列表查询会 SQL 报错，故 CH DDL/迁移必须同步加列（Task 1 覆盖）。
- **免费模型与预扣费失败边界**：免费模型（`priceData.FreeModel`）必须有占位行（插入点在分支前）；价格计算/敏感词/预扣费失败的请求不得产生占位行。

---

### Task 1: Log 模型 status 列 + 状态常量 + 迁移（含 ClickHouse）

**Files:**
- Modify: `model/log.go`（`Log` struct 增 `Status` 字段 + 四个 `LogStatus*` 常量；本任务不实现 pending/finalize 函数——Task 2）
- Modify: `model/main.go`（`clickHouseLogCreateTableSQL` 增 `status Int8 DEFAULT 0`；`migrateClickHouseLogDB` 增 `ALTER TABLE logs ADD COLUMN IF NOT EXISTS status Int8 DEFAULT 0`）
- Test: `model/log_status_migration_test.go`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces: `model.Log.Status int`（JSON `status`，gorm `default:0`）；`model.LogStatusCompleted=0`、`LogStatusInProgress=1`、`LogStatusFailed=2`、`LogStatusTimeout=3`；fixture（`model` 包 `TestMain` 的 `&Log{}` AutoMigrate + `truncateTables` 的 `DELETE FROM logs`）在后续任务复用。

- [ ] **Step 1: Write the failing test**（`model/log_status_migration_test.go`）

```go
func TestLegacyLogRowDefaultsToCompletedStatus(t *testing.T) {
	truncateTables(t)
	row := &Log{UserId: 1, Type: LogTypeConsume, ModelName: "legacy-model", Quota: 0}
	require.NoError(t, createLog(row).Error)
	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, row.Id).Error)
	assert.EqualValues(t, model.LogStatusCompleted, reloaded.Status) // 0
}
```
语义：插入未显式设置 status 的行，读回必须是 0（存量/未写占位的行全部按已完成）。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./model/ -run TestLegacyLogRowDefaultsToCompletedStatus -v`
Expected: FAIL（`Status` 字段不存在，编译不过或值为零值但无该字段——以编译失败为准）

- [ ] **Step 3: Implement**（`model/log.go`：`Log` 加 `Status int \`json:"status" gorm:"default:0"\``；加四个 `LogStatus*` 常量；`model/main.go`：CH DDL 加 `status Int8 DEFAULT 0`（放在 `quota Int32 DEFAULT 0` 之后），`migrateClickHouseLogDB` 在 `syncClickHouseLogTTL` 前加 `if err := LOG_DB.Exec("ALTER TABLE logs ADD COLUMN IF NOT EXISTS status Int8 DEFAULT 0").Error; err != nil { return err }`）

- [ ] **Step 4: Run tests**

Run: `go test ./model/ -run TestLegacyLogRowDefaultsToCompletedStatus -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add model/log.go model/main.go model/log_status_migration_test.go
git commit -m "feat: add log status column for in-progress consume logs"
```

---

### Task 2: model 层占位/终值化/清理函数

**Files:**
- Modify: `model/log.go`
- Test: `model/consume_log_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Log.Status` 与 `LogStatus*` 常量。
- Produces:
  - `func RecordPendingConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) (int, error)`——插入 `Status=LogStatusInProgress` 的行并强制 `Quota/PromptTokens/CompletionTokens/UseTime=0`（不管调用方传什么），返回 id（写失败返回 0 + error）。`params.Other` 由调用方（service）提供，至少含 `"in_progress": true`。
  - `func UpdateConsumeLogFinal(c *gin.Context, id int, params RecordConsumeLogParams) (bool, error)`——`WHERE id=? AND status=1` 更新给定 params 全部字段 + `status=0`；返回是否命中；**不命中时调用方负责回退插入**（model 不自己做回退）。
  - `func UpdateConsumeLogFailed(c *gin.Context, id int, useTimeSeconds int, channelId int, other map[string]interface{}) (bool, error)`——`WHERE id=? AND status=1` 置 `status=2`、`use_time`、`channel_id`、`other`（quota/tokens 保持 0）。
  - `func SweepStaleInProgressLogs(beforeUnix int64) (int64, error)`——`UPDATE logs SET status=3 WHERE type=2 AND status=1 AND created_at < ?`，返回影响行数。
  - 三处终值化成功后调用既有 `LogQuotaData` 导出（`UpdateConsumeLogFinal`/`UpdateConsumeLogFailed` 内部做，pending 插入不导出）。

- [ ] **Step 1: Write the failing tests**（`model/consume_log_test.go`，表驱动或分用例）

```go
// pending 行默认字段
func TestRecordPendingConsumeLogDefaults(t *testing.T)   // status=1、quota/tokens/use_time=0、other 含 "in_progress":true
// 守卫：对已完成行再次 Final 不生效
func TestUpdateConsumeLogFinalGuard(t *testing.T)        // pending->final 命中；对已 final 行再调返回 false 且字段不变
// 缺失行返回 false（供 service 回退）
func TestUpdateConsumeLogFinalMissingRowReturnsFalse(t *testing.T)
// 失败终值化只动 status/use_time/channel/other，不写 tokens/quota
func TestUpdateConsumeLogFailedKeepsTokensZero(t *testing.T)
// 清理只清超时 pending，不动已完成/失败/新 pending
func TestSweepStaleInProgressLogsOnlyStalePending(t *testing.T)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./model/ -run "TestRecordPendingConsumeLogDefaults|TestUpdateConsumeLogFinal|TestSweepStaleInProgressLogs" -v`
Expected: FAIL（函数未定义，编译失败）

- [ ] **Step 3: Implement**（`model/log.go`：四个函数。注意 `RecordPendingConsumeLog` **不自己拼 other**（service/log_info_generate.go 的 `appendRequestPath` 不在 model 包）——`params.Other` 由调用方给；本函数只强制 status=1 与零值计费字段。更新用 `Updates(map[string]interface{}{...})` + `Model(&Log{}).Where("id = ? AND status = ?", id, LogStatusInProgress)`；成功后调用 `RecordConsumeLog` 内已有的 `LogQuotaData` 导出代码路径——抽成 `exportQuotaData(userId, username, params, createdAt)` 小 helper 供插入与更新两处复用。）

- [ ] **Step 4: Run tests**

Run: `go test ./model/ -run "TestRecordPendingConsumeLogDefaults|TestUpdateConsumeLogFinal|TestUpdateConsumeLogFailed|TestSweepStaleInProgressLogs|TestUpdateConsumeLogFinalMissingRowReturnsFalse" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add model/log.go model/consume_log_test.go
git commit -m "feat: add pending, finalize and sweep helpers for consume logs"
```

---

### Task 3: RelayInfo 字段 + service 生命周期 + Relay() 接线

**Files:**
- Modify: `relay/common/relay_info.go`（`ConsumeLogId int`、`ConsumeLogFinalized bool`、`ConsumeLogRetryFailures []RetryFailureRecord`、`LastRetryWaitMs int`、`RetryFailureRecord` 类型与 `MaxRetryFailureRecords = 20` 常量——放 `relay/common` 避免 service 依赖环）
- Create: `service/consume_log.go`
- Modify: `controller/relay.go`（插入占位行 + defer 失败终值化）
- Test: `service/consume_log_test.go`

**Interfaces:**
- Consumes: Task 2 的 `model.RecordPendingConsumeLog`/`UpdateConsumeLogFinal`/`UpdateConsumeLogFailed`；Task 1 常量。
- Produces:
  - `func ShouldWriteInProgressLog() bool` = `common.LogConsumeEnabled && common.InProgressLogEnabled && !common.UsingLogDatabase(common.DatabaseTypeClickHouse)`。
  - `func RecordPendingConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo) int`——返回行 id；Realtime 格式、开关关闭、CH 返回 0。
  - `func finalizeConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, userId int, params model.RecordConsumeLogParams)`——`ConsumeLogId>0 && !ConsumeLogFinalized` 时调 `UpdateConsumeLogFinal`，命中即置 Finalized；未命中（被清理）回退 `model.RecordConsumeLog` 并 `SysLog` 告警；无 id 时直接 `RecordConsumeLog`。params.Other 非空时合并 `relayInfo.ConsumeLogRetryFailures`（Task 4）。
  - `func MarkConsumeLogFailed(c *gin.Context, relayInfo *relaycommon.RelayInfo, err *types.NewAPIError)`——`ConsumeLogId>0 && !ConsumeLogFinalized` 时置 Finalized 并调 `UpdateConsumeLogFailed(id, useTime, channelId, other)`，other 含 `retry_failures`、`failure_reason`、`admin_info.retry_attempts`。
  - `controller.Relay()` 内调用点：line ~160（`ModelPriceHelper` 成功后）`relayInfo.ConsumeLogId = service.RecordPendingConsumeLog(c, relayInfo)`；defer 内 `Refund` 之后加 `service.MarkConsumeLogFailed(c, relayInfo, newAPIError)`（`newAPIError != nil` 分支内）。

- [ ] **Step 1: Write the failing test**（`service/consume_log_test.go`——纯逻辑，不碰 DB）

```go
func TestShouldWriteInProgressLog(t *testing.T) {
	// 三态：默认开；LogConsumeEnabled=false -> false；UsingLogDatabase(CH) -> false
	// 用 common.SetDatabaseTypes(common.DatabaseTypeSQLite, CH) 构造第三种
}
```

- [ ] **Step 2: Run to verify fail**：`go test ./service/ -run TestShouldWriteInProgressLog -v` → FAIL（函数未定义）

- [ ] **Step 3: Implement**：`common.InProgressLogEnabled` 变量本任务在 `common/constants.go` 定义（默认 true）+ `InProgressLogSweepAfterSeconds = 43200` 常量（option 管线下任务接）；`relay_info.go` 加字段与类型；`service/consume_log.go` 实现三个函数；`controller/relay.go` 两处接线。

- [ ] **Step 4: Run**：`go test ./service/ -run TestShouldWriteInProgressLog -v` → PASS；并 `go build ./common/... ./relay/... ./service/... ./controller/...` 确认编译。

- [ ] **Step 5: Commit**

```bash
git add common/constants.go relay/common/relay_info.go service/consume_log.go service/consume_log_test.go controller/relay.go
git commit -m "feat: insert pending consume log at relay start and finalize on failure"
```

---

### Task 4: 每次失败尝试的错误日志收敛

**Files:**
- Modify: `service/consume_log.go`（`AppendRetryFailure` + 合并进 finalize/MarkConsumeLogFailed 的 other）
- Modify: `controller/relay.go`（`processChannelError` 门控；`waitBeforeRetry` 记录 `LastRetryWaitMs`）
- Test: `service/consume_log_test.go`（追加用例）

**Interfaces:**
- Consumes: Task 3 的 RelayInfo 字段与 lifecycle 函数。
- Produces:
  - `func AppendRetryFailure(relayInfo *relaycommon.RelayInfo, channelError types.ChannelError, err *types.NewAPIError)`——组装 `RetryFailureRecord{RetryIndex, ChannelId, ChannelName, StatusCode, ErrorCode, Reason: common.LocalLogPreview(err.MaskSensitiveErrorWithStatusCode()), WaitedMs: relayInfo.LastRetryWaitMs}`，达到 20 条置 `relayInfo.ConsumeLogRetryFailuresTruncated = true`（RelayInfo 增该 bool 字段）。
  - `controller/relay.go` `processChannelError`：`types.IsRecordErrorLog(err)` 且 `common.ErrorLogEnabled` 且 `relayInfo.ConsumeLogId > 0 && !relayInfo.ConsumeLogFinalized` → 只 `service.AppendRetryFailure(...)`，**不调** `model.RecordErrorLog`；否则维持原 `RecordErrorLog`。
  - `waitBeforeRetry` 实际等待时写 `relayInfo.LastRetryWaitMs`（`Relay()` 循环内可访问 relayInfo——把 `relayInfo` 作为参数传入或改包级 helper 签名：`waitBeforeRetry(c, relayInfo, statusCode, remainingRetry)`）。

- [ ] **Step 1: Write the failing tests**（service 包，纯逻辑）

```go
func TestAppendRetryFailureCapsAndTruncates(t *testing.T)
// 21 条：前 20 条入列、第 21 条被丢弃且 Truncated=true
func TestAppendRetryFailureRecordFields(t *testing.T)
// 断言 RetryIndex/ChannelId/ChannelName/StatusCode/ErrorCode/Reason(带脱敏预览)/WaitedMs
```

- [ ] **Step 2: Run to verify fail**：`go test ./service/ -run "TestAppendRetryFailure" -v` → FAIL

- [ ] **Step 3: Implement**（`AppendRetryFailure`；`processChannelError` 门控；`waitBeforeRetry` 加 relayInfo 参数与 `LastRetryWaitMs`；Truncated 字段）

- [ ] **Step 4: Run**：`go test ./service/ -run TestAppendRetryFailure -v` → PASS；`go test ./controller/ -run TestWaitBeforeRetry -v`（既有用例回归）→ PASS

- [ ] **Step 5: Commit**

```bash
git add service/consume_log.go service/consume_log_test.go controller/relay.go
git commit -m "feat: fold per-attempt failures into the request consume log row"
```

---

### Task 5: 结算点改走 finalizeConsumeLog

**Files:**
- Modify: `service/text_quota.go`（`PostTextConsumeQuota` 的 `model.RecordConsumeLog` 调用，line ~526）
- Modify: `service/quota.go`（`PostWssConsumeQuota` line ~245、`PostAudioConsumeQuota` line ~368）
- Test: `service/consume_log_test.go`（追加：合并 retry_failures 进 other）

**Interfaces:**
- Consumes: Task 3/4 的 `finalizeConsumeLog`、`relayInfo.ConsumeLogRetryFailures`。
- Produces: 三个结算函数在 `params` 组装完之后统一改为 `finalizeConsumeLog(ctx, relayInfo, relayInfo.UserId, params)`；`finalizeConsumeLog` 在 `params.Other != nil` 时把非空的 `relayInfo.ConsumeLogRetryFailures` 写为 `params.Other["retry_failures"]`（截断标记写 `retry_failures_truncated`）。

- [ ] **Step 1: Write the failing test**

```go
func TestFinalizeConsumeLogMergesRetryFailures(t *testing.T)
// 直接调 buildRetryFailuresOther 或等价合并逻辑（纯函数）：非空列表 -> other["retry_failures"] = 列表；空列表 -> 不写 key
```

- [ ] **Step 2: Run to verify fail**：`go test ./service/ -run TestFinalizeConsumeLogMergesRetryFailures -v` → FAIL

- [ ] **Step 3: Implement**：合并逻辑抽成 service 内纯函数 `mergeRetryFailures(other map[string]interface{}, relayInfo *relaycommon.RelayInfo)`；三处结算点换用 `finalizeConsumeLog`。

- [ ] **Step 4: Run**：`go test ./service/ -run TestFinalizeConsumeLogMergesRetryFailures -v` → PASS；`go build ./service/...`；`go test ./controller/ ./relay/... 2>&1 | tail -5` 快速回归。

- [ ] **Step 5: Commit**

```bash
git add service/text_quota.go service/quota.go service/consume_log.go service/consume_log_test.go
git commit -m "feat: finalize the pending consume log row on settlement"
```

---

### Task 6: 选项管道、清理任务、统计与列表过滤

**Files:**
- Modify: `common/constants.go`（Task 3 已加变量，本任务加 `MigrationNote` 无；只需确认默认值）
- Modify: `model/option.go`（`InitOptionMap` 默认 `InProgressLogEnabled` + `updateOptionMap` case）
- Create: `service/consume_log_cleanup.go`（`StartInProgressLogCleanup`）
- Modify: `main.go`（`service.StartAuthArtifactCleanup()` 旁调用）
- Modify: `model/log.go`（`GetAllLogs`/`GetUserLogs` 增 `status int` 尾参与动态 Where；所有 `type=LogTypeConsume` 聚合加 `status=LogStatusCompleted`；`rg "LogTypeConsume"` 全枚举确认）
- Modify: `controller/log.go`（`GetAllLogs`/`GetUserLogs`/`/self` 变体解析 `status` query：空串 → -1）
- Test: `model/option_in_progress_test.go`（or 追加到已有 option 测试文件）、`model/consume_log_test.go`（列表过滤）、`controller/log_status_filter_test.go`（query 解析）

**Interfaces:**
- Consumes: Task 2 的 `SweepStaleInProgressLogs`；Task 1 常量。
- Produces:
  - option key `InProgressLogEnabled`（bool，默认 true）与 `common.InProgressLogEnabled` 双向同步。
  - `func StartInProgressLogCleanup()`：master-only、启动即跑 + 每 1 小时，阈值 `common.InProgressLogSweepAfterSeconds`。
  - `model.GetAllLogs(..., upstreamRequestId string, status int)` / `model.GetUserLogs(..., upstreamRequestId string, status int)`：`status >= 0` 时加 `tx = tx.Where("logs.status = ?", status)`；`-1` 不过滤。
  - controller 解析 helper `parseLogStatusQuery(c *gin.Context) int`（空 → -1，非空 Atoi）。
  - `GetLogByTokenId`、`GetLogsStat`/`GetLogsSelfStat` 同口径（stat 加 `status=0` 过滤或透传 status，按 Spec §7.4：聚合加 status=0）。

- [ ] **Step 1: Write the failing tests**

```go
// model
func TestGetUserLogsStatusFilter(t *testing.T)              // 造 1 pending + 1 completed：-1 全返、1 只返 pending、0 只返 completed
func TestSumUsedQuotaExcludesNonFinalLogs(t *testing.T)     // quota 求和只计 status=0
// controller
func TestParseLogStatusQuery(t *testing.T)                  // 空->-1；"1"->1；"abc"->-1
// option
func TestUpdateOptionMapInProgressLogEnabled(t *testing.T)  // "false" -> common.InProgressLogEnabled=false；非法值保持 true
```

- [ ] **Step 2: Run to verify fail**（三个包分别跑）→ FAIL

- [ ] **Step 3: Implement**（按 Interfaces；`main.go` 在 `service.StartAuthArtifactCleanup()` 后加 `service.StartInProgressLogCleanup()`；stat 类聚合 `rg "LogTypeConsume"` 逐一加 `status = 0`——`model/log.go` 的 `SumUsedQuota`、per-model/group token 统计等）

- [ ] **Step 4: Run**：三包测试全 PASS；`go build ./...`（排除 root embed 限制时用 `go build ./common/... ./setting/... ./model/... ./service/... ./controller/... ./relay/...`）。

- [ ] **Step 5: Commit**

```bash
git add model/option.go model/log.go model/option_in_progress_test.go model/consume_log_test.go controller/log.go controller/log_status_filter_test.go service/consume_log_cleanup.go main.go
git commit -m "feat: wire in-progress log option, stale sweep and status filters"
```

---

### Task 7: 前端（schema/筛选/单元格/详情/自动刷新）

**Files:**
- Modify: `web/src/features/usage-logs/data/schema.ts`（`status: z.number().default(0)`）
- Modify: `web/src/features/usage-logs/types.ts`（`GetLogsParams` 增 `status?: number`；`CommonLogFilters` 增 `status?: number`）
- Modify: `web/src/features/usage-logs/lib/utils.ts`（`buildApiParams` 透传 `status`；`fetchLogsByCategory` 无需改）
- Modify: `web/src/features/usage-logs/components/common-logs-filter-bar.tsx`（状态 Select + 自动刷新 Switch，URL search 读写）
- Modify: `web/src/features/usage-logs/components/timing-metrics-cell.tsx`（`StreamTpsCell`/`TimingMetricsCell` 加 `isPending`/`status` 占位渲染）
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`（Tokens 列占位；渠道列 `retry_attempts>1` 显示“重试 N 次”小标）
- Modify: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`（渲染 `other.retry_failures` 与 `failure_reason`）
- Modify: `web/src/features/usage-logs/components/usage-logs-mobile-card.tsx`（同源适配）
- Modify: `web/src/features/usage-logs/components/usage-logs-table.tsx`（`useQuery` 加 `refetchInterval`；autoRefresh 状态与 filter bar 联动）
- Modify: `web/src/i18n/locales/*.json`（经 `add-missing-keys.mjs` + `bun run i18n:sync`，7 语言；随后删临时脚本）
- Test: `web/src/features/usage-logs/components/__tests__/stream-tps.test.tsx`（追加 pending 用例）、`web/src/features/usage-logs/lib/__tests__/throughput.test.ts` 旁新增 `__tests__/log-filter-params.test.ts`（`buildApiParams` status 透传）

**Interfaces:**
- Consumes: Task 6 的接口（`status` query 参数）。
- Produces: 列表行对 `status=1` 显示“请求中”徽标 + 全部数值列占位；`status=2` 显示“失败”；URL `status` 搜索参数（值 `-1` 省略、`0/1/2/3`）。

- [ ] **Step 1: Write the failing tests**

```tsx
// stream-tps.test.tsx 追加
test('shows the in-progress placeholder instead of throughput', ...)
// render(<StreamTpsCell isStream tokensPerSecond={null} tokensPerSecondAfterFirstToken={null} isPending />)
// expect 'In progress'（en bundle 注入）与 '—' 在文档中
// log-filter-params.test.ts
test('forwards the status filter to the logs API', ...)
// buildApiParams({ searchParams: { status: 1 } }) -> { status: 1 }；status 缺省 -> 不含该 key
```

- [ ] **Step 2: Run to verify fail**：`npx -y bun run test -- src/features/usage-logs/components/__tests__/stream-tps.test.tsx src/features/usage-logs/lib/__tests__/` → FAIL

- [ ] **Step 3: Implement**（按 Files 清单；i18n 键（含插值的键要保留 `{{...}}`）：`In progress` / `Failed` / `Timed out` / `Status` / `Auto refresh` / `Retried {{count}} attempts` / `Failure reason` / `Retry history`，7 语言；`refetchInterval` 规则：`autoRefresh && (status 筛选为全部 or 请求中)` → 5000，否则 false）

- [ ] **Step 4: Run**：两个测试文件 PASS；`npx -y bun run typecheck`（预期既有 2 个 channel-form.ts 错误外无新增）；`.\node_modules\.bin\oxlint.exe -c .oxlintrc.json <改动文件>` exit 0；`oxfmt --check <改动文件>` 通过。

- [ ] **Step 5: Commit**

```bash
git add web/src/features/usage-logs web/src/i18n/locales
git commit -m "feat: show in-progress consume logs in the usage logs UI"
```

---

### Task 8: 系统设置开关 + 端到端校验

**Files:**
- Modify: `web/src/features/system-settings/types.ts`、`models/index.tsx`（`InProgressLogEnabled` 默认 true）、`operations/section-registry.tsx`、`operations/log-settings-section.tsx`、`operations/index.tsx`
- Test: `web/src/features/system-settings/operations/__tests__/`（如该目录不存在则并入现有 operations 测试；最少覆盖表单默认值与提交 payload）

**Interfaces:**
- Consumes: Task 6 的 option key `InProgressLogEnabled`。
- Produces: 日志与统计设置页一个 Switch，label `In-progress request logs`，描述说明写放大与默认开启。

- [ ] **Step 1: Write the failing test**（schema/默认值断言）

- [ ] **Step 2: Run to verify fail**

- [ ] **Step 3: Implement**（镜像 `LogConsumeEnabled` 表单接入方式）

- [ ] **Step 4: Run**：相关测试 + typecheck/lint/format；后端 `go test ./model/ ./service/ ./controller/ ./setting/operation_setting/` 全套回归；前端 `npx -y bun run test -- src/features/system-settings src/features/usage-logs`。

- [ ] **Step 5: Commit + 端到端人工核对清单**（提交后在 PR/交付说明里列出本地核对步骤：开启 → 发一个会失败的请求 → 日志出现“请求中”后翻成“失败”；重试请求只有一行且 `retry_failures` 有条目；关闭开关后不再出现占位行且旧行仍被终值化）

```bash
git add web/src/features/system-settings
git commit -m "feat: add in-progress consume log setting"
```

---

## Self-Review Notes

- Spec 覆盖：§7.1 开关（T6）、§7.2 插入/成功/失败终值化（T3/T5）、§7.3 清理（T2/T6）、§7.4 统计与列表（T6）、§7.5 错误日志收敛（T4/T5）、§12 CH 禁用+加列（T1/T3/T6）、前端（T7/T8）、Realtime 例外（T3 `RecordPendingConsumeLog` 内 `relayInfo.RelayFormat == types.RelayFormatOpenAIRealtime` 跳过）。
- 边界：免费模型（T3 插入点=价格计算后、FreeModel 分支前）；预扣费前失败无行（T3 接线位置决定）；`InProgressLogEnabled` 缺省 -1（T6 controller helper 测试）。
- 类型一致性：`finalizeConsumeLog`/`MarkConsumeLogFailed`/`AppendRetryFailure`/`ShouldWriteInProgressLog`/`mergeRetryFailures` 在 T3/T4/T5 使用同一签名；`UpdateConsumeLogFinal`/`UpdateConsumeLogFailed`/`SweepStaleInProgressLogs` 在 T2 定义后只被 T3/T6 调用。
