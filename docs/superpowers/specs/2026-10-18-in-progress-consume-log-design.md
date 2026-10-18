# 进行中消耗日志（In-progress consume log）设计

日期：2026-10-18
状态：待评审

## 1. 意图与动机

现状（已核实代码）：

- 消耗日志只在**结算时**写一条：`service/text_quota.go` 的 `PostTextConsumeQuota`、`service/quota.go` 的 `PostWssConsumeQuota` / `PostAudioConsumeQuota` 最终都调用 `model.RecordConsumeLog`（`model/log.go`）。这三处覆盖 controller.Relay 下全部文本类端点（chat/claude/gemini/responses/image/embedding/rerank/alpha_search/audio/wss）。
- **失败请求完全不写消耗日志**（只退还预扣费）；按渠道尝试逐条写的是**错误日志**（`type=5`，`controller/relay.go` `processChannelError` → `model.RecordErrorLog`），N 次失败尝试 = N 条错误日志。
- 预扣费在重试循环外只做一次，所以**今天的消耗日志本来就是一个请求一条**（仅成功且有 usage 时）。

用户诉求：请求一进入 new-api 就有一条消耗日志（“流”列显示“请求中”，Tokens/耗时用占位符），结束时同一条精准更新；一次请求无论重试多少次都只有一条日志。

已拍板的决策（用户）：

1. 只做**结束时一次性精准更新**，不做流式中途的增量更新。
2. 失败请求也保留一条 `status=失败`、`quota=0` 的行，在消耗日志列表中可见。
3. 第一期范围 = `controller.Relay` 下的所有文本类端点；任务类（Video/Midjourney）本期不做。
4. 总开关默认**开启**。

## 2. 目标

1. 预扣费成功后立刻落库一行 `status=请求中`；请求结束时（成功或失败）原地更新为精准终值（`status=已完成` 或 `status=失败`）。
2. 一次请求无论多少次渠道重试/重试间隔等待，消耗日志始终只有一行；`other.admin_info` 中记录 `use_channel` 列表与重试次数，`other.retry_failures` 记录每次失败尝试的脱敏摘要（错误日志不再按尝试逐条写，见 §7.5）。
3. 前端支持按状态筛选（全部 / 已完成 / 请求中 / 失败 / 已超时），请求中态使用占位展示，并提供自动刷新。
4. 存量日志与统计口径不受影响：`status=0`（默认值）表示"已完成/存量行"，所有聚合只统计 `status=0`。
5. 节点崩溃或多节点部署下残留的"请求中"行，由 master 定期兜底标记为"已超时"。

## 3. 非目标（Non-goals）

- 流式中途的增量更新（用户明确不做）。
- 任务类端点（`RelayTask` / MJ）本期不做。
- 错误日志按请求收敛已在本期范围内（见 §7.5）；**预-Relay 阶段的错误日志**（鉴权、限流、参数校验等）保持今天的逐条行为。
- 不改预扣费/退还/结算等计费语义。

## 4. 反目标（Anti-goals）

- 不把 pending 行只放在内存里等结算才落库——必须落库，否则崩溃与多节点下不可见，也违背"请求进站就写日志"的初衷。
- 不让聚合统计把未完成行计入（quota 求和、请求数、TPS 等必须排除 `status != 0`）。
- 不引入会让存量行语义改变的 status 取值：`0` 必须永远是"已完成/存量未知行"。

## 5. 约束

- 三库兼容（SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6）；SQLite 迁移用 `ALTER TABLE ... ADD COLUMN`，照 `model/main.go` 既有迁移模式。
- ClickHouse 日志库上整个功能禁用（见 §12）。
- status 用 `int` + 显式赋值，不用 GORM 布尔类默认值 tag 导致反复 ALTER（遵循 AGENTS.md）。
- 不新增 status 索引：清理按 `created_at` 过滤、筛选查询已有 `created_at`/`user_id` 索引覆盖（实现时用 EXPLAIN 复核，若证据不足再加）。
- 写放大：每请求 insert + update 两次写。提供开关可关，并让部署者知情。
- 所有日志写失败只记 `common.SysError`，不影响请求主流程与计费。

## 6. 数据模型

### 6.1 model.Log 新增字段

```go
// relay/common 或 model/log.go
Status int `json:"status" gorm:"default:0"`
```

常量（放在 `model/log.go`，与现有 `LogType*` 同风格，追加不用 iota 保有旧值）：

```go
const (
	LogStatusCompleted  = 0 // 已完成（含所有存量行）
	LogStatusInProgress = 1 // 请求中（占位行）
	LogStatusFailed     = 2 // 失败（全部尝试均失败）
	LogStatusTimeout    = 3 // 兜底清理标记的超时
)
```

### 6.2 迁移

`model/main.go` 按既有 SQLite/MySQL/PG 迁移模式追加 `status` 列（`INTEGER/INT DEFAULT 0`，MySQL/PG 由 AutoMigrate 或显式 ALTER 处理，SQLite 用 `ALTER TABLE logs ADD COLUMN ...`）。存量行自然成为 `status=0`。

## 7. 后端

### 7.1 开关与配置

- `common.InProgressLogEnabled bool`（默认 `true`），option key `InProgressLogEnabled`。
- `model/option.go`：`InitOptionMap` 默认值 + `updateOptionMap` case，与 `LogConsumeEnabled` 完全同管道（common 变量 ↔ OptionMap ↔ 前端开关）。
- 生效条件：`common.LogConsumeEnabled && common.InProgressLogEnabled`（前者已存在：总日志开关）。

### 7.2 请求生命周期

**插入占位行**（`controller/relay.go` `Relay()`）：

- 位置：`helper.ModelPriceHelper` 成功之后、`priceData.FreeModel` 分支**之前**（即免费模型也覆盖；`PreConsumeBilling` 的 if/else 分支保持不动，插入点同时服务付费与免费两条路径）。
- `service.RecordPendingConsumeLog(c, relayInfo)` → `model.RecordPendingConsumeLog(...)`，返回行 id 写入 `relayInfo.ConsumeLogId`（`relay/common/relay_info.go` 的 `RelayInfo` 新增字段 `ConsumeLogId int`、`ConsumeLogFinalized bool`）。
- 占位行字段：
  - `Type=LogTypeConsume`、`Status=LogStatusInProgress`
  - `PromptTokens/CompletionTokens/Quota/UseTime=0`、`Content=""`
  - `ChannelId=relayInfo.ChannelId`（首个尝试的渠道，可能为 0）
  - `ModelName=relayInfo.OriginModelName`（原始客户端模型名，映射/前缀处理前）
  - `TokenName/TokenId/UserId/Username/Group/IsStream/Ip/RequestId/UpstreamRequestId` 与终值相同来源
  - `Other`：至少 `{"in_progress": true}`，并复用 `appendRequestPath` 写入 `request_path`
- 写失败只 `SysError` 并返回 0，后续一切按原逻辑。

**成功终值化**（3 个结算函数，均改为"优先更新占位行"）：

覆盖：

- `service/text_quota.go` `PostTextConsumeQuota`
- `service/quota.go` `PostWssConsumeQuota`
- `service/quota.go` `PostAudioConsumeQuota`

统一走一个新 helper（`service` 包内新建 `service/consume_log.go`）：

```go
func finalizeConsumeLog(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, userId int, params model.RecordConsumeLogParams)
```

- `relayInfo.ConsumeLogId > 0 && !relayInfo.ConsumeLogFinalized` → `model.UpdateConsumeLog(ctx, id, params)`，并置 `ConsumeLogFinalized=true`。
- 否则 = 现状行为 `model.RecordConsumeLog`（channel-test、MJ、任务、违规费、日志关闭等旁路全部不变）。

`model.UpdateConsumeLog`：

```go
func UpdateConsumeLog(c *gin.Context, id int, params RecordConsumeLogParams) error {
	// 用 Updates + map 写全：status->0、quota、prompt/completion tokens、use_time、
	// channel_id、model_name、content、other、is_stream；WHERE id=? AND status=1
	// RowsAffected==0（被兜底清理/异常丢失）→ 回退 RecordConsumeLog 插入已完成行 + SysLog 告警
	// 成功更新后执行 LogQuotaData 导出（占位插入时不导出）
}
```

守卫 `WHERE status=1` + `ConsumeLogFinalized` 双重保证一次终值化；回退插入保证不丢账。

**失败终值化**（`controller/relay.go` 既有 defer 错误分支扩展）：

- `newAPIError != nil` 且 `relayInfo.ConsumeLogId > 0` 且未终值化 → `service.MarkConsumeLogFailed(c, relayInfo, newAPIError)`：
  - `status -> 2`、`UseTime=int(time.Since(startTime).Seconds())`（`startTime` 取 `constant.ContextKeyRequestStartTime`）、`ChannelId=relayInfo.ChannelId`。
  - `Other` 合并：`other.admin_info.use_channel`（与现有错误日志/成功日志同源同位置，`formatUserLogs` 对非管理员仍会剥掉）、`other.admin_info.retry_attempts=relayInfo.RetryIndex+1`、`other.failure_reason`（顶层，脱敏短错误，与错误日志掩码口径一致——非管理员需要知道自己失败的原因）。
  - quota/tokens 保持 0。
- 与既有 `ChargeViolationFeeIfNeeded`（`service/violation_fee.go` 自己 Insert 一条违规费用行）互不干扰，保持原样。

**OpenAI Realtime（WSS）例外**：`/v1/realtime` 是长连接 WebSocket，没有自然的"请求结束"边界且可以合法超过任何清理阈值，本期**不插占位行**（保持现状：socket 关闭时由 `PostWssConsumeQuota` 写一条终值日志）。在 `RecordPendingConsumeLog` 入口对 `relayFormat == types.RelayFormatOpenAIRealtime` 直接跳过（记 debug 日志）。

**边界**：

- 预扣费之前失败的请求（鉴权、限流、无可用渠道前的参数校验等）**不**产生占位行——它们本来就没有消耗日志，且有错误日志。
- 预扣费成功后、首个渠道都没选到的失败（如"分组下无可用渠道"）也会留下 `status=失败` 行（这正是新增的可观测性）。
- Playground 请求同样有占位行（与现有消耗日志行为一致）。

### 7.5 每次失败尝试的错误日志收敛（已决策：收敛）

现状：`controller/relay.go` 的 `processChannelError` 对每个失败尝试写一条 `type=5` 错误日志（`model.RecordErrorLog`），所以 N 次失败 = N 条错误日志。

新规则：

- **有 pending 行时**（`relayInfo.ConsumeLogId > 0 && !relayInfo.ConsumeLogFinalized`）：不再写 `type=5` 行，改为 `service.AppendRetryFailure(relayInfo, channelError, err)` 累积到 `relayInfo.ConsumeLogRetryFailures[]`，条目：`{retry_index, channel_id, channel_name, status_code, error_code, reason（脱敏短错误，与现有错误日志同掩码口径）, waited_ms（本次重试前等待的间隔，0 表示无）}`。
- **终值化时**把列表落进同一行：成功走 `PostTextConsumeQuota`/`PostWssConsumeQuota`/`PostAudioConsumeQuota` 的 `finalizeConsumeLog`（写入 `other.retry_failures`），失败走 `MarkConsumeLogFailed`（写入 `other.retry_failures` + `other.failure_reason`=最后一条）。成功但有重试的行同样展示重试链路（这正是用户最关心的可见性）。
- **无 pending 行时**（开关关闭、ClickHouse、插行失败、预扣费前失败、Realtime）保持今天的逐条错误日志，一行不改。
- `constant.ErrorLogEnabled` 仍是总开关：关闭时既不写错误日志也不累积（与今天语义一致）。
- `types.IsRecordErrorLog(err)` 的豁免同样适用于累积（与今天的错误日志口径一致）。
- 渠道自动禁用（`ShouldDisableChannel` → `DisableChannel`）逻辑不受影响，继续在 `processChannelError` 内异步执行。
- 可见性：`retry_failures` 放 `other` 顶层（脱敏文案与客户端实际收到的错误一致，对请求归属用户可见，非敏感）；展示在详情弹窗。
- 规模保护：最多累积 20 条，超出置 `other.retry_failures_truncated=true`。
- 实现时 `rg "RecordErrorLog"` 复核唯一调用点是 `processChannelError`；channel-test 等旁路不变。
- 前端：详情弹窗渲染 `retry_failures`（重试序号/渠道/状态码/原因/等待）；列表行在 `admin_info.retry_attempts > 1` 时于渠道列显示"重试 N 次"小标。

### 7.3 兜底清理

- 新建 `service/consume_log_cleanup.go`：`StartInProgressLogCleanup()`，完全镜像 `service/auth_cleanup.go` 的 `StartAuthArtifactCleanup`（master only、启动即执行一次 + 定时器）。
- 间隔 1 小时。
- `model.SweepStaleInProgressLogs(beforeUnix int64)`：`UPDATE logs SET status=3 WHERE type=2 AND status=1 AND created_at < ?`；条数记 SysLog。
- 阈值常量 `common.InProgressLogSweepAfterSeconds = 43200`（12 小时，远大于任何合法流式请求时长；被误清理的行在终值化时会走"回退插入已完成行"，不丢账）。
- `main.go` 在 `service.StartAuthArtifactCleanup()` 旁新增调用。

### 7.4 统计与查询

- 所有对消耗日志（`type=LogTypeConsume`）的**聚合**追加 `status=LogStatusCompleted` 条件。实现时 `rg "LogTypeConsume"` / `rg "type = .2"` 全量枚举，逐一确认，包括但不限于：`model/log.go` 的 token 统计查询（655/691 附近）、usedata 概览、dashboard 各统计端点。
- **列表查询**新增可选 `status` 参数（`-1`=不过滤，默认不过滤，让"全部"仍能看到请求中/失败；`0/1/2/3` 精确过滤）。覆盖：`GetUserLogs`/`GetAllLogs`（`controller/log.go`）、`/self` 变体、`GetLogByTokenId`（令牌详情页）。"tabs 角标"与列表同口径，不过滤 status。
- ClickHouse 日志库见 §12。

## 8. 前端

- `features/usage-logs/data/schema.ts`：日志 schema 增加 `status: z.number().default(0)`。
- 筛选栏（filters）：新增「状态」（全部 / 已完成 / 请求中 / 失败 / 已超时），映射为 `status` 查询参数。
- `StreamTpsCell`（stream/tps 单元格）新增 `isPending`：pending 时把"流/非流"徽标替换为「请求中」（带轻微呼吸圆点），TPS 显示占位 `—`；失败态显示「失败」徽标。
- `TimingMetricsCell` pending 时首字/耗时均占位 `—`。
- Tokens 列（`common-logs-columns.tsx`）pending 时两数字占位 `…`；详情弹窗展示 status、`failure_reason`、`use_channel`、`retry_attempts`。
- 移动端卡片（`usage-logs-mobile-card.tsx`）同源适配。
- 自动刷新：日志工具栏新增开关（默认开），**仅当状态筛选包含"请求中"（全部或请求中）时**以 5s 轮询，其它情况下不轮询。
- 系统设置 → 日志与统计：新增开关「进行中请求日志」（镜像 `LogConsumeEnabled` 表单与 defaults），option key `InProgressLogEnabled`，默认 true。
- i18n：新增英文键（`In progress` / `Failed` / `Timed out` / `Status` / `Auto refresh` / 「请求中」相关描述等 7-8 条）走 `add-missing-keys.mjs` + `bun run i18n:sync`，7 语言齐全。

## 9. 测试

后端（`require`/`assert` 风格，表驱动）：

- `model`：`RecordPendingConsumeLog` 默认字段（status=1、quota/tokens/use_time=0、other.in_progress）；`UpdateConsumeLog` 更新语义与 `WHERE status=1` 守卫（对已完成行重复更新不生效）；`RowsAffected=0` 回退插入；`SweepStaleInProgressLogs` 只清超时 pending 行、不动其他行。
- `service`：`finalizeConsumeLog`（有 id → 更新；无 id → 走原插入）；`MarkConsumeLogFailed` 幂等（重复调用不二次更新）。
- 聚合口径：存在 pending/failed 行时统计接口不把它们计入（覆盖 §7.4 的回归）。
- 错误日志收敛：有 pending 行且重试 1 次时，`logs` 表中该请求只有 1 行（消耗）且 `other.retry_failures` 有 1 条；无 pending 行（插行失败/开关关闭）时错误日志仍按尝试逐条；`retry_failures` 超 20 条置 truncated；`ErrorLogEnabled=false` 两者都不写。
- 使用仓库既有 model 测试夹具初始化数据库。

前端：

- `StreamTpsCell` pending 渲染（RTL：显示"请求中"与占位，不显示 tps）。
- status 筛选参数装配 zod/表测量（如走 zod schema）。
- 跑 typecheck、oxlint、oxfmt 与相关测试文件。

## 10. 兼容性与回滚

- 关闭 `InProgressLogEnabled`：不再插占位行，行为回到现状；已插入但未终值化的行仍会被结算点按 id 更新（或由清理任务兜底），**不会丢账**。
- 开关可在请求进行中随时切换：插入和终值化分别判断，互不依赖。
- 存量行 `status=NULL/0` 全部按已完成处理，无数据修复脚本。
- 免费模型（`priceData.FreeModel`）同样有占位行（插入点在分支之前，见 §7.2）。
- OpenAI Realtime（WSS）不插占位行，行为与今天一致。

## 11. 已决策事项

- **ClickHouse 日志库**：`common.UsingLogDatabase(common.DatabaseTypeClickHouse)` 为真时整个功能禁用（不插占位行、`RecordErrorLog` 保持逐条、列表不过滤 status、前端状态筛选降级）。理由：CH 为 append-only 列存，点更新需变异 SQL，与"每请求一次点更新"相性差；这类部署保持今天的"结算时一条"行为。
- **错误日志按请求收敛**：见 §7.5。一次请求的每次失败尝试不再各写一条 `type=5` 错误日志，而是汇入该请求那一行的 `other.retry_failures`。

## 12. ClickHouse 日志库

`LOG_DB` 支持 ClickHouse（建表 DDL 在 `model/main.go` 的 `clickHouseLogCreateTableSQL`），其模型是 append-only 的列存：点更新要走 `ALTER TABLE logs UPDATE ...` 变异（mutation），和本功能"每请求一次点更新"的用法相性很差，且会显著拖慢 CH 集群。

**决策（已定，方案 A）**：`common.UsingLogDatabase(common.DatabaseTypeClickHouse)` 为真时**整个功能禁用**（不插占位行、列表不过滤 status、错误日志保持逐条、前端状态筛选降级）。理由：CH 通常就是大规模场景，insert+update 写放大与点更新变异都不合其模型；这些部署保持今天的"结算时一条 + 逐条错误日志"行为。

**实现要点（builder 复查补充）**：`model.Log` 加 `Status` 字段后，所有列表查询 `SELECT` 都会带上 `logs.status`，所以 CH 侧**必须**把列加上，否则 CH 部署的日志列表直接报错：

- `clickHouseLogCreateTableSQL` 增 `status Int8 DEFAULT 0`；
- `migrateClickHouseLogDB` 增 `LOG_DB.Exec("ALTER TABLE logs ADD COLUMN IF NOT EXISTS status Int8 DEFAULT 0")`（CH 元数据操作，轻量，存量表可用）；
- `ShouldWriteInProgressLog()` 在 CH 下返回 false（不写 pending 行），其余行为完全保持现状。

## 13. 文件级改动清单（给 builder）

后端：

- `common/constants.go`：`InProgressLogEnabled`、`InProgressLogSweepAfterSeconds`。
- `common/constants.go` option 默认值注册：同 `LogConsumeEnabled`（若它在 constants 里默认注册）。
- `model/log.go`：`Status` 字段、status 常量、`RecordPendingConsumeLog`、`UpdateConsumeLog`、`SweepStaleInProgressLogs`。
- `model/main.go`：迁移。
- `model/option.go`：默认导出 + case。
- `relay/common/relay_info.go`：`ConsumeLogId`/`ConsumeLogFinalized`/`ConsumeLogRetryFailures` 字段。
- `service/consume_log.go`（新）：`RecordPendingConsumeLog`、`finalizeConsumeLog`、`MarkConsumeLogFailed`、`AppendRetryFailure`。
- `service/consume_log_cleanup.go`（新）：`StartInProgressLogCleanup`。
- `service/text_quota.go`、`service/quota.go`：结算点改走 `finalizeConsumeLog`。
- `controller/relay.go`：插入 + defer 失败终值化 + `processChannelError` 错误日志收敛（§7.5）。
- 日志聚合查询（rg 枚举）补 `status=0`；列表查询补 `status` 过滤参数（controller + model）。
- 测试（§9）。

前端：

- `features/usage-logs/data/schema.ts`、筛选栏、三个单元格组件、详情弹窗（含 `retry_failures` 渲染与"重试 N 次"小标）、移动端卡片、自动刷新开关。
- `features/system-settings`：`types.ts`、`defaults`、`operations/log-settings-section.tsx`（镜像 LogConsumeEnabled）、注册表。
- i18n 7 语言 + 新测试。
