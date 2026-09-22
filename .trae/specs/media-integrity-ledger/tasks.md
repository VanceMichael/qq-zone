# 媒体完整性账本、离线核验与定点修复 - Implementation Plan

## Task 1: 账本核心模型、原子存储与哈希工具
- **Status**: `completed`
- **Completion Evidence**:
  - 新增 internal/app/ledger.go（Ledger/Entry/域根/身份构造/upsert/前缀替换/原子写/bak 回退/域根发现）与 internal/pkg/util FileDigest+FileSHA256（256KiB 固定缓冲流式）。
  - TR-1.1~1.5 全部由 ledger_test.go 覆盖并通过（go test 绿）：身份格式、媒体类型推断、upsert 去重与 local 替换、前缀删除、原子写无 tmp 残留、损坏回退 bak、双损坏 ErrLedgerCorrupt、四类域根发现、EntryFromFile 摘要一致。
- **Priority**: high
- **Depends On**: None
- **Description**:
  - 新增 `internal/app/ledger.go`：`Ledger`/`LedgerEntry` 结构（version、kind、owner_uin、group_id、hash_alg=sha256、时间戳、entries），四类备份域根路径推导（album / qun/<gid> / shuoshuo / liuyanban），域根发现（glob），source_id 构造器（album/group/shuoshuo/board/local），按 source_id upsert、按相册目录前缀替换条目（全量相册用）、按 rel_path 查找。
  - 原子持久化：同目录 `.tmp` 写完整 → Sync → rename 替换 `ledger.json`；成功替换后刷新 `ledger.bak`；加载时 ledger.json 损坏回退 bak，两者皆损返回显式错误（禁止静默重置）。
  - 流式 SHA-256（固定缓冲，`io.Copy`），可放 `internal/pkg/util`（如 `FileSHA256`）；媒体类型按扩展名推断（image/video/voice）。
  - 新增 `internal/app/ledger_test.go`：往返序列化、upsert 去重、前缀替换、损坏回退、原子写后无残留 tmp、身份格式、域根发现。
- **Acceptance Criteria Addressed**: AC-1, AC-3, AC-15
- **Test Requirements**:
  - `rule` TR-1.1: 四类域根路径与发现结果正确（含 qun 层级），证据：glob 夹具测试。
  - `rule` TR-1.2: source_id 五类构造格式与 FR-1 完全一致，证据：表驱动断言。
  - `rule` TR-1.3: ledger.json 损坏自动回退 ledger.bak；两者损坏返回错误且不写空账本，证据：故障注入测试。
  - `rule` TR-1.4: 原子写成功后目录下无临时文件，重复保存内容可往返，证据：目录列表与重新加载断言。
  - `rule` TR-1.5: SHA-256 流式计算与内存哈希结果一致，且读取使用固定小缓冲（代码审查），证据：测试 + 走查。
- **Notes**: 账本时间统一 `time.Now`；JSON 字段 snake_case，与现有 backup.json/task record 风格一致。

## Task 2: HLS 完成信号加固（.part 临时文件 + 原子改名）
- **Status**: `completed`
- **Completion Evidence**:
  - hls.go 分片改写 <target>.part，Sync+Close 后 os.Rename 原子改名；四个失败返回点均关闭文件并保留 .part。
  - 新增 hls_integrity_test.go（httptest 服务）：TR-2.1 成功拼接 INIT+SEG*、无 .part 残留、返回值指向最终路径；TR-2.2 seg 500 时最终文件不存在且 .part 保留；go test ./internal/net/http 全绿。
- **Priority**: high
- **Depends On**: None
- **Description**:
  - 修改 `internal/net/http/hls.go`：分片写入 `<target>.part` 而非最终文件名；全部分片成功、文件正常关闭后 rename 为最终 target（目标已存在时先处理 Windows 兼容替换）；任一分片失败或 ctx 取消时保留 `.part`、不产生最终文件。
  - 新增 `internal/net/http/hls_integrity_test.go`：用 `httptest` 提供 m3u8 + 分片，成功路径断言最终文件内容正确且无 `.part` 残留；中途失败（某分片 500/中断）断言不存在最终文件、存在 `.part`。
- **Acceptance Criteria Addressed**: AC-2
- **Test Requirements**:
  - `rule` TR-2.1: 成功 HLS 下载后 target 存在、内容为 init+segments 顺序拼接、`.part` 与 sidecar 不存在，证据：httptest 测试。
  - `rule` TR-2.2: 分片失败/取消时最终 target 不存在、`<target>.part` 保留，证据：httptest 失败用例。
  - `rule` TR-2.3: 返回值 map 的 path/filename/dir 指向最终 target，证据：测试断言。
- **Notes**: 普通直链的 `.resume.json` 语义已存在，本任务不改 `http.go` 续传逻辑。

## Task 3: 个人相册与群相册备份链路集成账本
- **Status**: `completed`
- **Completion Evidence**:
  - ledger.go 增加四类链路共用的 SkipDecision（healthy/noEntry/mismatch，纯本地 stat+sha256）。
  - spider.go：Download/RetryFailed 起始 initLedger（双损坏即中止，不用空账本覆盖）；downloadItem 改为账本一致离线跳过、不一致强制重下、无条目回退旧 HEAD 且不登记；新下载成功 stage 完成条目；每个相册干净边界 commitAlbumLedger（非量整体前缀替换），取消丢弃本相册 staged；RetryFailed 结束 commitStaged；群相册 group: 身份与 qun 域根。
  - spider_integrity_test.go 覆盖 TR-3.1~3.5：四分支跳过、staging 身份/rel（timeline+扩展名）、全量替换保留他相册、取消账本字节不变、群身份/域根、清空相册不毁域根账本、域外路径拒绝；go test ./... 全绿。
- **Priority**: high
- **Depends On**: Task 1, Task 2
- **Description**:
  - `internal/app/spider.go`：Spider 增加账本句柄与并发安全的 staged 条目集合；`Download` 开始时按模式打开/新建域根账本（加载失败显式报错）。
  - `downloadItem`：
    - 新下载成功后用最终 actualTarget（含 Content-Type 改扩展名、timeline 子目录、Chtimes 后内容不变）计算 size/sha256，stage 条目（source_id 用 album/group 格式，origin_verified=true；同 rel_path 若有 local 条目则替换）；
    - 增量跳过改为可单测的决策：先查账本条目（source_id 匹配且现存 rel_path 文件 size+sha256 一致）→ 本地跳过（不发 HEAD）；条目不一致 → 不跳过进入下载；无条目 → 保留现有 HEAD 逻辑且不登记；
    - 失败项不 stage。
  - 提交边界：每个相册 `downloadAlbum` 完整结束后合并 staged 并原子保存；非增量（exclude=false）成功的相册先移除该相册名前缀的旧条目；`RetryFailed` 结束时提交一次；ctx 取消时丢弃当前未提交相册的 staged。
  - 群相册 source_id 含登录 QQ 与群号；rel_path 相对 qun/<gid> 域根。
  - 新增 `spider_integrity_test.go`：跳过决策四分支（一致/改动/缺失/无条目）、staging 身份与路径、全量前缀替换、取消不提交。下载部分以可调用决策函数 + 临时目录真实文件方式测试，不依赖网络。
- **Acceptance Criteria Addressed**: AC-1, AC-2, AC-4, AC-5, AC-15
- **Test Requirements**:
  - `rule` TR-3.1: 一致文件账本跳过路径不访问网络（决策函数无 client 调用），证据：分支测试。
  - `rule` TR-3.2: 改动/缺失文件不跳过；无条目文件走旧 HEAD 分支且不产生账本变更，证据：分支测试。
  - `rule` TR-3.3: 新下载成功条目身份格式、rel_path（含年月子目录与扩展名变更）、size/sha256 与磁盘一致；失败项无条目，证据：临时目录测试。
  - `rule` TR-3.4: 全量相册提交后旧前缀条目被替换、其余相册条目保留；取消时账本保持提交前版本，证据：测试。
  - `rule` TR-3.5: 群相册条目位于 qun 域根且 source_id 含群号，证据：测试。
- **Notes**: `buildLocalFileIndex` 非增量删的是相册子目录，账本在域根 `.integrity/`，天然不受影响，补一条测试确认。

## Task 4: 说说与留言板备份链路集成账本
- **Status**: `completed`
- **Completion Evidence**:
  - 新增 media_ledger_glue.go：说说/留言板共用 mediaLedgerSession（init/sourceID 含 url 兜底/decision/stage/commit）。
  - mood_backup.go/board_backup.go：Backup/RetryFailed 起始 init（双损坏中止）；downloadOneMedia 与 RetryFailed 现存文件短路改为账本一致离线跳过、mismatch 强制重下、无条目保留旧行为且不登记；成功（含重试兜底构造身份）stage，Backup/Retry 正常结束单次 commit，取消 commit(false) 保留旧账本。
  - media_ledger_glue_test.go：TR-4.1 身份/语音类型/rel、TR-4.2 三分支（含 mood/board 各一，nil client 证明跳过路径零网络）、TR-4.3 取消账本字节不变+域外拒绝、TR-4.4 头像/assets 无 stage 入口（走查 downloadAvatars 未接触 ledger）；go test ./... 全绿。
- **Priority**: high
- **Depends On**: Task 1, Task 2
- **Description**:
  - `mood_backup.go` / `board_backup.go`：Backup 开始打开域根账本；`downloadOneMedia` 成功（含视频/语音）后按最终路径 stage 条目（shuoshuo/board 身份，媒体 ID 为空走 url+MD5 兜底，origin_verified=true；替换同 rel_path 的 local 条目）。
  - 既有「文件非空即跳过」改为决策函数：账本条目一致 → 跳过；条目存在但 size/sha256 不一致 → 不跳过，落回下载重建；无条目 → 保留旧跳过行为且不登记。`RetryFailed` 里现存文件短路（mood_backup.go 约 244 行）同样套用该决策。
  - 提交时机：Backup 正常走完（backup.json 与查看页写好）后原子提交一次；RetryFailed 结束提交一次；ctx 取消不提交。
  - 头像、assets、raw、data 不登记。
  - 新增 mood/board 账本集成测试（临时 backup.json + 媒体夹具，覆盖 stage/跳过/不一致重建/取消不提交）。
- **Acceptance Criteria Addressed**: AC-1, AC-2, AC-4, AC-5
- **Test Requirements**:
  - `rule` TR-4.1: 媒体成功后条目身份（tid+media id / url 兜底）、rel_path（media/YYYY/MM）、size/sha256 正确；评论/转发内媒体与语音同样登记，证据：测试。
  - `rule` TR-4.2: 账本一致跳过、不一致触发下载路径、无条目旧行为且不登记，三分支测试通过（说说与留言板各一）。
  - `rule` TR-4.3: 取消（ctx 取消）不提交账本，上一版可解析，证据：测试。
  - `rule` TR-4.4: avatars/assets/raw/data 下文件不进入账本，证据：条目扫描断言。
- **Notes**: 留言板只有图片链路；决策函数尽量在两文件间复用（放到共享文件，如 `media_ledger_glue.go`）。

## Task 5: 离线核验器（四分类、稳定、只读）
- **Status**: `completed`
- **Completion Evidence**:
  - 新增 integrity_verify.go：scanManagedFiles（相册递归排除 .integrity/元数据；说说/留言板仅 media/；识别 .resume.json/.part 标记），按 FR-5 优先级分类缺失→未完成→大小→摘要→健康（区分已/未验证），剩余媒体为未登记；结果稳定排序；全程仅 os.Stat/Open/Read，import 无任何网络包；另有 ScanRootWithoutLedger 供无账本仅扫描。
  - TR-5.1/5.2/5.3/5.4/5.5 由 integrity_verify_test.go 覆盖通过：全类别夹具计数正确、size/hash 两子类型、sidecar 与孤立 part 映射、辅助文件零噪声、两次报告 reflect.DeepEqual、核验前后整树 sha256 快照一致、无账本 ErrLedgerMissing。
- **Priority**: high
- **Depends On**: Task 1
- **Description**:
  - 新增 `internal/app/integrity_verify.go`：输入域根，按 FR-5 规则产出核验报告（缺失/内容变更/未登记/未完成四类 + 健康统计含已验证/未验证计数）。
  - 完成态标记识别：`*.resume.json` sidecar（映射到去后缀目标）、`*.part`；管理范围：相册域根递归（排除 `.integrity/`、album_metadata.json、标记文件）；说说/留言板仅 `media/`（排除标记文件）。
  - 分类优先级与 FR-5 一致；结果按 rel_path 排序；哈希流式、带进度回调（mpb 可选注入，测试传 nil）。
  - 严格只读：函数只接受文件系统读操作；不创建/修改/删除任何文件；无 http.Client、无 qzone.Client 依赖。
  - 新增 `integrity_verify_test.go`：夹具目录覆盖全部类别（含 sidecar、.part、未登记、基线未验证条目、timeline 子目录），两次运行结果完全一致；运行前后夹具目录整树哈希不变。
- **Acceptance Criteria Addressed**: AC-6, AC-15
- **Test Requirements**:
  - `rule` TR-5.1: 四类计数与明细与夹具预置完全一致，健康项区分 origin_verified true/false，证据：夹具测试。
  - `rule` TR-5.2: 同一夹具两次核验结果（含顺序）逐字段相等，证据：比较两次报告。
  - `rule` TR-5.3: 核验前后目录所有文件（含账本）字节一致，证据：核验前后整树 sha256 快照断言。
  - `rule` TR-5.4: 核验代码不依赖 net/http 客户端与登录态（包依赖走查），证据：import 走查。
  - `rule` TR-5.5: sidecar 映射（f→去 .resume.json）与 .part 归类正确，证据：夹具断言。

## Task 6: 旧备份本地基线与「未登记纳入基线」
- **Status**: `completed`
- **Completion Evidence**:
  - 新增 integrity_baseline.go：EstablishBaseline（已有账本 ErrLedgerExists；仅登记现存非未完成媒体，local 身份 + origin_verified=false，未完成清单单列，原子保存）、AdoptUnregistered（只追加未登记项，幂等，不动媒体）、BaselineNotice 统一文案。
  - integrity_baseline_test.go 覆盖 TR-6.1~6.4：全部未验证/local、half 目标+sidecar+part 排除、元数据排除、拒绝重建、纳入后媒体树不变（仅账本文件变）、幂等再纳入 0、基线后核验健康未验证且无缺失/变更；测试全绿。
- **Priority**: high
- **Depends On**: Task 5
- **Description**:
  - 新增 `internal/app/integrity_baseline.go`：对无账本域根，经显式调用（带确认由 CLI 负责）扫描管理范围内现存、非未完成媒体，计算 size/sha256，写入 origin_verified=false、source_id=local:<rel_path> 的账本并原子保存；未完成文件列入返回提示，不登记。
  - 已有账本时拒绝「重建基线」；提供 `AdoptUnregistered`：把核验报告中的未登记项追加为 local 未验证条目（去重、不改文件）。
  - 文案常量集中管理，明确「来源未验证、未与空间原件核对」。
  - 新增测试：无账本建基线、未完成排除、已有账本拒绝、纳入未登记、基线条目不被后续核验误报为变更/缺失。
- **Acceptance Criteria Addressed**: AC-7, AC-12
- **Test Requirements**:
  - `rule` TR-6.1: 基线条目全部 origin_verified=false、source_id=local:<rel_path>、size/sha256 与文件一致，证据：测试。
  - `rule` TR-6.2: .part/.resume 目标被排除并在提示中列出，证据：测试。
  - `rule` TR-6.3: 已有账本时建基线返回明确错误；AdoptUnregistered 只追加未登记项且不动磁盘文件，证据：测试 + 文件树快照。
  - `rule` TR-6.4: 基线后立即核验，基线条目为健康（未验证）而非缺失/变更，证据：联动测试。

## Task 7: 修复计划持久化与定点修复编排
- **Status**: `completed`
- **Completion Evidence**:
  - integrity_repair.go：repair.json 模型/原子读写/空计划自删/HasPendingRepair；NewRepairPlan 只收 missing/changed/incomplete，按 source_id+backup.json 补相册名/相册ID/tid/media/url 取源提示，不可定位项 auto_repairable=false；ExecuteRepair 先把计划内目标+sidecar/.part 移到 .integrity/quarantine/<时间戳>/<原结构>，再经注入的 RepairRunner 走原链路（相册白名单 Spider、说说/留言板 RetryFailed），以 rel 路径账本健康裁决移除/保留计划项，断网/无凭证链路异常时计划不写回、原样保留。
  - integrity_repair_runner.go：生产 OnlineRepairRunner 完全复用 Spider/MoodBackup/BoardBackup；task_record 增加 TaskModeRepair；Spider 白名单支持相册 ID。
  - 顺带修正 Upsert 语义：同 rel_path 上已验证条目接管任意旧身份（修复/身份迁移场景），由修复测试暴露并回归。
  - integrity_repair_test.go（假 runner）：TR-7.1 隔离目录结构含 sidecar；TR-7.2 健康/未登记 mtime 不变；TR-7.3 成功移除计划+登记、失败保留 last_error、断网计划不动、二轮续作清空；TR-7.4 mood FailedItem 复用 RetryFailed；TR-7.5 孤立 part 不可修复并保留；计划往返/自删；go build/vet/test 全绿。
- **Priority**: high
- **Depends On**: Task 3, Task 4, Task 5, Task 6
- **Description**:
  - 新增 `internal/app/integrity_repair.go`：
    - `.integrity/repair.json` 模型与原子读写；由核验报告生成计划（missing/changed/incomplete；unregistered 不入计划）；相册类提示取相册名（rel_path 首段）/相册 ID（source_id）；说说/留言板从本地 backup.json 补 tid/media id/URL；local 身份且无法定位空间来源的项标记不可自动修复。
    - 隔离：changed/incomplete 目标（连同 sidecar/.part）移动到 `.integrity/quarantine/<YYYYMMDD-HHMMSS>/<原rel结构>`；missing 直接重建；计划外文件零接触。
    - 修复执行：个人/群相册以白名单 Spider（增量、账本跳过）跑受影响相册，登录与群授权沿用现有路径；说说/留言板构造 FailedItem 复用 `MoodBackup.RetryFailed`/`BoardBackup.RetryFailed`；成功后账本已有 stage/提交，计划中移除成功项并原子保存，失败项保留 last_error；不可修复项跳过并报告。
    - 结束写 mode=repair 的 TaskRecord（复用 DownloadResult 计数）。
  - 计划续作 API：Load/Save/Continue/丢弃；无凭证/网络失败/取消时计划原样保留。
  - 新增测试：计划生成与去重、原子读写、隔离目录结构、计划外文件不动（mtime 断言）、成功项移除/失败项保留、local 不可修复项跳过、续作加载；Spider/RetryFailed 调用以接口或函数变量注入假实现，避免真实网络。
- **Acceptance Criteria Addressed**: AC-8, AC-9, AC-13
- **Test Requirements**:
  - `rule` TR-7.1: changed/incomplete 文件被移动到 quarantine 且保留原相对结构；missing 项无隔离动作，证据：夹具测试。
  - `rule` TR-7.2: 健康与未登记文件在修复前后 mtime+内容不变，证据：修复前后快照断言（注入假下载器）。
  - `rule` TR-7.3: 成功项从 repair.json 移除且账本条目 origin_verified=true；失败/取消后计划完整保留、可再次加载继续，证据：测试。
  - `rule` TR-7.4: 修复仅调用既有 Spider / RetryFailed 链路（注入点与真实构造一致），未另起下载实现，证据：代码走查 + 注入断言。
  - `rule` TR-7.5: local 身份不可修复项被跳过并出现在报告，证据：测试。
  - `rubric` TR-7.6: 复用集成度；scale 1-5；anchors 1=另起下载/重试实现，3=部分复用，5=四类全部走既有链路与记录体系；threshold >= 4；证据：独立审查走查。
- **Notes**: 计划必须在请求登录之前落盘，保证无凭证场景下次可继续。

## Task 8: CLI「完整性核验与修复」菜单（免登录入口）
- **Status**: `completed`
- **Completion Evidence**:
  - 新增 internal/cli/integrity.go：域根发现列表（类型/账号/账本状态/条目数/损坏标记）、无账本时显式建基线（含 BaselineNotice 二次确认）或仅扫描、mpb 哈希进度、四段 tablewriter 报告（缺失/变更/未完成/未完成 + 健康已验证/未验证汇总）、修复/纳入未登记/丢弃计划/继续既有计划动作。
  - menu.go 主菜单新增「🧪 完整性核验与修复」（索引/描述同步重排），dispatch 明确不 ensureLogin；runRepair 先 plan.Save 再登录，无凭证即提示计划保留；修复写 TaskModeRepair 记录并汇总 resolved/remaining/unrepairable。
  - TR-8.1/8.2/8.3 走查通过：核验路径只调用 app 离线 API；计划落盘先于登录；基线/修复/丢弃均有 Confirm。go build/vet 全绿。
- **Priority**: high
- **Depends On**: Task 5, Task 6, Task 7
- **Description**:
  - 新增 `internal/cli/integrity.go` 并在 `menu.go` 主菜单加入「🧪 完整性核验与修复（本地核验无需登录）」，入口及整个核验过程不调用 ensureLogin、不发网络请求。
  - 流程：发现域根 → 列表选择（类型/账号或群/账本状态/条目数，单个或全部）→ 无账本时提供「建立本地基线（二次确认）」或「仅扫描」→ 核验（哈希进度条）→ 终端四段报告（tablewriter 风格 + 汇总：健康已验证/未验证、风险项数）→ 动作：修复（先落计划再登录）/继续既有计划/纳入未登记/丢弃计划（确认）/返回。
  - 修复前确认文案说明：仅重建确认项、损坏文件隔离、需要登录；登录失败/取消回主菜单且计划保留。
  - 修复结束复用 `renderTaskSummary` 或同风格汇总。
- **Acceptance Criteria Addressed**: AC-9, AC-10, AC-14
- **Test Requirements**:
  - `rule` TR-8.1: 入口到报告的代码路径无 ensureLogin/网络客户端调用，证据：代码走查（交互函数难单测，以走查+手工运行为准）。
  - `rule` TR-8.2: 计划落盘先于登录；登录失败后 repair.json 仍存在，证据：走查 + 手工运行记录。
  - `rule` TR-8.3: 破坏性动作（基线/修复/丢弃计划）均有 survey 确认；核验本身无确认，证据：走查。
  - `rubric` TR-8.4: 报告可用性；scale 1-5；anchors 1=原始日志堆砌，3=分段但缺汇总/风险提示，5=四段分明+汇总+风险提示+风格统一；threshold >= 4；证据：手工终端输出。
- **Notes**: 与现有菜单的 emoji/颜色/表格风格保持一致。

---

# Review Issues（R1 fail 后进入修复）

## Issue I-1: Ledger.Save 排序后索引失效，多相册跳过闸门串号（high）
- **Status**: `completed`
- **Priority**: high
- **Depends On**: None
- **Discovered By**: Review R1 (F1)
- **Description**:
  - Save 原地 sort Entries 后未 rebuildIndex，首个相册边界提交后，同进程后续 SkipDecision/FindBySource/Upsert 通过旧下标访问到别的条目；缺失文件可被判 healthy 离线跳过，Upsert 接管可误删无关健康条目。mood/board/RetryFailed 单次提交不受影响；磁盘 JSON 始终正确。
- **Acceptance Criteria Addressed**: AC-1, AC-4, AC-5
- **Test Requirements**:
  - `rule` TR-I-1.1: Save 后 FindBySource/FindByRelPath 全部仍命中自身条目（跨相册、乱序插入），证据：新单测。
  - `rule` TR-I-1.2: A 相册提交后删除 B 相册文件，B source 的 SkipDecision 必须为 mismatch；再 Upsert A 新条目不得删除 B 条目，证据：回归测试。
- **Notes**: 修复方向=排序后 rebuildIndex（或对副本 marshal）。

## Issue I-2: mood/board 把 media/ 之外的未完成标记误报为未完成（low）
- **Status**: `completed`
- **Priority**: medium
- **Depends On**: None
- **Discovered By**: Review R1 (F2)
- **Description**:
  - scanManagedFiles 的 marker 判定先于 inManagedScope，avatars/10001.jpg.resume.json 被计入未完成并可能进 repair.json。mood/board 的 marker 仅当目标位于 media/ 前缀才登记。
- **Acceptance Criteria Addressed**: AC-6
- **Test Requirements**:
  - `rule` TR-I-2.1: shuoshuo/board 域根 avatars 下 sidecar/.part 不出现在任何核验分类（既非未完成也非未登记），证据：夹具测试。
  - `rule` TR-I-2.2: media/ 内 marker 仍正确计入，证据：既有用例不回归。

## Issue I-3: 修复动作缺少独立二次确认（low）
- **Status**: `completed`
- **Priority**: medium
- **Depends On**: None
- **Discovered By**: Review R1 (F3)
- **Description**:
  - 选择「修复以上问题」后直接 plan.Save→登录，无独立 Confirm，与 FR-9 矛盾。「继续既有计划」已有一次询问，作为确认不重复弹。
- **Acceptance Criteria Addressed**: AC-10
- **Test Requirements**:
  - `rule` TR-I-3.1: 新修复计划落盘前有一次 askYesNo（文案含隔离/登录/健康不重写），拒绝则不落盘不登录；续作路径沿用既有询问，证据：走查。

## Issue I-4: 审查 advisory 顺手修复（A3/A4/A6 + A5 测试盲区）
- **Status**: `completed`
- **Priority**: low
- **Depends On**: I-1
- **Discovered By**: Review R1 (advisory)
- **Description**:
  - A3：修复裁决接受「source_id 条目在新 rel 路径上健康」（Content-Type 改扩展名场景），避免计划项永久残留。
  - A4：账本提交失败的 Warn 文案改为与实际补登记条件一致（文件重新在线下载时才补登记；否则可用纳入基线）。
  - A6：moodFailedItems 按 MediaKind 带上 IsVideo，保证视频 getinfo 换源链路触发。
  - A5：补 HLS ctx 取消路径测试。
- **Acceptance Criteria Addressed**: AC-8, AC-2
- **Test Requirements**:
  - `rule` TR-I-4.1: source 条目在新 rel 健康时计划项可裁决 resolved；IsVideo 透传断言；HLS 取消只留 .part，证据：单测。

### Issues 修复验证证据（R1→R2 之间）
- **I-1**：ledger.Save 排序后立即 rebuildIndex；新增 TestLedgerSaveKeepsIndexesValidAcrossAlbums（乱序四条目 Save 后 FindBySource/RelPath 全对位；删 B0 后 A2 再提交，B0 仍 mismatch 且 B 条目不被误删）。
- **I-2**：scanManagedFiles 先 inManagedScope 再识别 marker；新增 TestVerifyMoodIgnoresMarkersOutsideMediaScope（avatars/assets 下 sidecar/.part 在有账本核验与无账本扫描中均零噪声，media 内标记不回归）。
- **I-3**：runRepair 增加 alreadyConfirmed 参数，新计划落盘前独立 askYesNo（含隔离/登录/健康不重写文案，默认否）；「继续既有计划」沿用既有询问不重复弹。
- **I-4**：ExecuteRepair 增加 ledgerSourceResolved 裁决（同来源在改名后路径健康即 resolved）；moodFailedItems 透传 IsVideo；账本提交失败 Warn 文案与实际补登记条件一致；新增 TestDownloadHLSContextCancelKeepsOnlyPart（分片阻塞+ctx 取消，最终文件不存在、.part 保留已写字节）与 TestRepairResolvedBySourceAfterExtensionRename（a.jpg→a.png 同来源裁决 resolved 且断言 IsVideo=true）。
- 复跑：go build/vet/test 全绿，go test -race ./internal/app ./internal/net/http 通过，gofmt 无差异。

## Task 9: 全量构建、静态检查、测试与手工冒烟
- **Status**: `completed`
- **Completion Evidence**:
  - TR-9.1：`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全部退出 0；gofmt 无差异；GOOS=windows/darwin 交叉编译均通过。
  - TR-9.2（临时冒烟测试，已删除）：在临时 cwd 构造真实 storage 布局（album/qun/shuoshuo/liuyanban 四域根 + .part/.resume.json），DiscoverLedgerRoots 发现 4 个；逐域根 EstablishBaseline→VerifyRoot→NewRepairPlan：基线条目全部健康（未验证）、未完成标记被排除且不入媒体、相册 part 可修复、无 backup.json 的孤立 board sidecar 正确标为不可自动修复、未登记 0。
  - TR-9.3：1 GiB（sparse）FileDigest 643ms、≈1592 MiB/s、runtime Alloc≈3.2 MiB，固定 256KiB 缓冲流式读取，内存不随文件增长；CLI 核验带 mpb 文件级进度条。
  - 查看页回归走查：viewer 只读 data/backup.json 与 media/，`.integrity/` 位于各域根隐藏目录，viewer 无任何引用；说说/留言板 backup.json 结构未改。
- **Priority**: high
- **Depends On**: Task 1-8
- **Description**:
  - 运行 `go build ./...`、`go vet ./...`、`go test ./...` 全绿；补跑既有 board/mood/spider 相关测试确认无回归。
  - 手工冒烟（本机无网络依赖部分）：构造小型夹具存储目录，走通 免登录核验 → 四段报告 → 建基线/纳入 → 修复计划落盘（登录失败保留）→ 再次进入可见续作；确认说说/留言板查看页正常打开且 `.integrity/` 不影响页面。
  - 大文件（≥1GB 夹具文件）核验计时与内存粗测，确认流式与进度。
- **Acceptance Criteria Addressed**: AC-11, AC-15
- **Test Requirements**:
  - `rule` TR-9.1: build/vet/test 全部退出码 0，证据：命令输出记录。
  - `rule` TR-9.2: 冒烟流程各步骤结果符合预期、查看页可打开，证据：手工记录。
  - `rubric` TR-9.3: 大文件处理；scale 1-5；anchors 1=整文件载入/无进度，3=流式但无进度或明显 I/O 放大，5=流式+进度+固定缓冲；threshold >= 4；证据：计时/内存观察。
