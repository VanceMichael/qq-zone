# 在相册写盘前冻结下载计划并校验容量

> **Workspace:** /Users/vance/project/mineProject/swe/092113/project-03

---

**User**

多年相册写入容量有限的目标盘时，下载开始之后才发现空间不足会留下任务记录、半成品和难以对账的执行结果。为个人相册和群相册增加写盘前的备份预检：用户选好账号或群、相册及现有备份选项后，程序先拉取完整的分页媒体清单，按与真正下载相同的目标路径、时间线和本地索引规则，把每项分为新下载、完整跳过、可续传和大小未知，并在确认前展示各类数量、已知剩余字节、未知项以及目标卷可用空间。续传项只计尚未写入的字节，HEAD 失败、HLS 或无可靠长度的媒体要显式列为未知，不能当成零字节。已知剩余字节大于目标卷可用空间时直接拒绝，必须在任务记录、目录、临时文件或媒体内容产生之前结束；容量查询失败也要显式报错并保持零写入。分页中途失败使整份计划无效，恢复网络后需重新规划；单个大小探测失败只增加未知项，用户明确确认后仍可执行。确认后的备份必须消费刚才展示的同一份冻结清单，不得因二次拉取而静默加入新照片；下载地址过期时可用现有解析链更换同一媒体的取源地址，不能更换媒体身份。用户在预检或确认阶段取消时不留任务和文件副作用；不经过本次新预检的其他既有备份、查看和历史失败重试流程保持原有行为。

**Assistant**

我先了解项目结构，再决定如何实施这个备份预检功能。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/092113/project-03

这是一个 QQ 空间备份的 Go 项目。让我深入了解群相册、个人相册备份的现有架构。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/group.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/qzone/group.go

继续了解下载引擎、个人相册备份流程和任务记录。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/menu.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/task_record.go

继续了解 HTTP 层（HEAD/Download/HLS）、视频源解析和个人相册的清单接口。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/http.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/qzone/video.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/hls.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/qzone/qzone.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/config.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/progress.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/qzone/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/util.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider_group_test.go

已基本掌握架构。再确认日志工厂的副作用、平台文件构建标签风格和 go 版本。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/logger/logger.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/stdin_other.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/stdin_windows.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/go.mod

我已经完整理解了现有架构。这是一个涉及多文件的复杂功能，先建立任务清单，并确认其他备份链路对 Spider 的依赖。

toolName: todo_write

status: success

Todos updated: 10 items

toolName: search_by_regex

status: success

边界已确认：`Spider.Download/DownloadPlan` 仅被个人/群相册两个 CLI 流程调用，说说、留言板、失败重试都是独立链路，不受影响。开始实现。

**第一步：HTTP 层增加只读的体积探测与续传状态查询。**

toolName: todo_write

status: success

Todos updated: 10 items

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/http.go

changes: undefined

**第二步：跨平台磁盘容量查询（只读，目录不存在时向上回退到现存祖先）。**

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk_other.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk_windows.go

changes: undefined

**第三步：重构 Spider——抽取预检与下载共用的命名/时间线/候选源规则。**

toolName: todo_write

status: success

Todos updated: 10 items

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

changes: undefined

现在拆分 `downloadAlbum`，让冻结清单可以直接复用相册处理主体。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

changes: undefined

**第四步：创建备份预检核心 `backup_plan.go`（冻结清单、零写入规划、四项分类）。**

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

修正两处：去掉多余 import 与探测中的重复请求，并给 Spider 增加测试用的清单拉取 seam。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

给 Spider 结构体补上测试用的清单拉取 seam 字段。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go

changes: undefined

先快速编译 app 与 http、util 包，及早发现问题。

toolName: Shell

status: success

app 层编译通过。**第五步：CLI 预检报告渲染与确认闸门（新建 precheck.go）。**

toolName: todo_write

status: success

Todos updated: 10 items

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/precheck.go

changes: undefined

接入个人相册流程（自己 + 好友都走 `handleSpider`）。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/menu.go

changes: undefined

接入群相册流程，并把 `RememberGroup`、任务日志与记录全部后移到确认之后。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/cli/group.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 10 items

toolName: Shell

status: success

编译与 vet 全部通过。**第六步：补充单元测试。**

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/probe_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk_test.go

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan_test.go

changes: undefined

清理测试里一行遗留的占位代码。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan_test.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 10 items

toolName: Shell

status: success

测试夹具问题：小响应体被 Go 的 http 服务自动补了 Content-Length。需要显式 Flush 强制 chunked 才能模拟“无可靠长度”。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/probe_test.go

changes: undefined

toolName: Shell

status: success

测试全绿。但我自查发现一处与真实下载规则的偏差需要修正：增量「完整跳过」判定必须与 `shouldSkipExisting` 完全对齐（只带 cookie 的 HEAD、不做 Range GET 兜底、跳过 getinfo/HLS），而容量计字节可以用 Range GET 的可靠长度。否则可能出现“计划判跳过、执行却重下”的低估。重构探测层，把两种依据分开。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/http.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

changes: undefined

补一个锁定该语义的测试：cookie-HEAD 被拒、仅 Range GET 能拿到长度时，即使本地文件等长也不能判跳过（真实下载会删文件重下）。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan_test.go

changes: undefined

toolName: Shell

status: success

全部通过。再做 Windows 交叉编译验证（新增了 `disk_windows.go`）和格式检查。

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

格式与测试全部通过。最后通读一遍新文件与两个接入点，确认逻辑闭环。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 10 items

备份预检功能已完成，全量 `go build`（含 Windows/Linux 交叉编译）、`go vet`、`go test` 均通过。下面是按需求逐条对应的实现说明。

## 整体流程

个人相册（自己/好友共用 [menu.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/menu.go#L434-L468) 的 `handleSpider`）和群相册（[group.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/group.go#L168-L211) 的 `backupOneGroup`）现在统一为：

**选群/账号 → 选相册 → 现有备份选项 → 预检（零写入）→ 容量报告 → 明确确认 → 才创建任务记录/日志/目录并执行冻结清单**

## 需求对照

1. **写盘前完整分页清单 + 同规则分类**：[backup_plan.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go#L114-L235) 的 `BuildBackupPlan` 逐相册调 `GetPhotoList`/`GetGroupPhotoList` 拉完全部分页；路径、时间线、文件名规则由下载与预检共享的 [buildMediaSpec](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go#L484-L552) 推导，本地索引走与下载完全一致但只读的 [readOnlyMediaIndex](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go#L422-L435)/[plannedMediaDir](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go#L400-L419)（不 `MkdirAll`、不 `RemoveAll`）。

2. **四类划分**（[planOneItem](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go#L248-L337)）：新下载（计完整长度）、完整跳过（与 `shouldSkipExisting` 同源的仅-cookie HEAD 且远端长度 ≤ 本地）、可续传（sidecar 绑定同一 URL 且是最高优先级候选，**只计 `远端长度−本地已有`**）、大小未知。半成品无 sidecar 或 URL 不匹配时，因下载层会先删后重下，保守按整份新下载计。

3. **未知不当零字节**：HEAD 失败、仅 HLS/getinfo 源、chunked 无 `Content-Length` 全部显式记 `unknown`（[ProbeSize](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/http.go#L212-L237) HEAD 后再做 `bytes=0-0` 探测，206 取 `Content-Range` 总长，拿不到就报错）。

4. **容量门槛**：预检先查卷（[VolumeFreeSpace](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk.go#L17-L39)，跨平台 unix/windows，路径不存在时向上找现存祖先）；查询失败直接报错；`KnownRemaining > FreeBytes` 在 [precheck.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/precheck.go#L55-L61) 硬拒绝，此时任务记录、任务日志、目录、临时文件、媒体均未产生。

5. **分页失败整份作废**：任一页失败即返回错误并丢弃内存计划，CLI 退出该流程；网络恢复后重走预检重新规划。单项探测失败只增加未知项，确认后仍可执行（确认页有黄色警示）。

6. **冻结清单消费**：新增 [DownloadPlan](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go#L246-L276)，只遍历 `plan.Albums` 里的相册/照片，不再二次拉列表（测试 `TestBuildBackupPlanFreezesAndWritesNothing` 断言执行期清单调用次数不增加）。下载地址过期时，仍由现有 `GetVideoSource` 浮层链 / `ResolveTencentVideo(vid)` 按同一 `sloc` 换 URL，不换媒体身份。

7. **取消零副作用**：预检中 Ctrl+C、确认选“否”都在 `createTaskLogger`/`NewTaskRecord`/`RememberGroup` 之前返回；群号记忆也从原来的选项页之前后移到确认之后。

8. **既有流程不变**：`Spider.Download`/`RetryFailed` 保留原行为，说说、留言板、查看页、历史失败重试均未改动。

## 主要文件

- 新增：[backup_plan.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/backup_plan.go)、[precheck.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/precheck.go)、[disk.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/pkg/util/disk.go)/disk_other.go/disk_windows.go 及 3 个测试文件
- 修改：[spider.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/app/spider.go)（抽共享规则、拆 `processAlbumPhotos`、新增 `DownloadPlan`/`SelectAlbums`）、[http.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/net/http/http.go)（探测与 `ResumeSidecar`）、[menu.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/menu.go)、[group.go](file:///Users/vance/project/mineProject/swe/092113/project-03/internal/cli/group.go)

测试覆盖了四分类、续传差值、无 sidecar 重下、HEAD 不可用不许跳过、HLS 未知、探测失败未知、非增量重下、分页失败作废且零写入、冻结执行不二次拉取等 13 个用例。

一个值得注意的设计取舍：个人相册每个视频在预检时要走一次浮层解析（受全局限速约 2 次/秒，与真实下载时的解析成本相同），多年相册中视频很多时预检阶段会相应变慢，终端有“拉取清单/探测大小”进度可见；如果后续觉得需要，可以再加“预检视频只计未知、不预解析”的加速选项。
