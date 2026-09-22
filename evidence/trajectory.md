# 为本地媒体副本建立可离线核验的完整性账本

> **Workspace:** /Users/vance/project/mineProject/swe/092113/project-02

---

**User**

备份盘被移动、文件被手工整理或经历异常断电后，用户现在无法分辨本地照片和视频是仍然可信，还是已缺失、被改写或只留下未完成文件。请在个人相册、群相册、说说和留言板四类备份中建立持久的媒体完整性账本：每个真正完成并关闭的目标文件都以稳定的来源身份、实际相对路径、字节数和内容摘要登记，续传半成品、HLS 中间产物或已失败项不得被标记为健康。提供不登录、不访问网络的本地核验入口，用稳定结果分开报告缺失、内容变更、未登记和未完成项，核验本身不修改文件或账本。用户选择修复后，程序应按原备份类型的现有取源和重试链路只重建被确认损坏或缺失的项，健康文件不得重写；无登录凭证、网络失败或用户取消时保留待修复结果，下次运行可继续。账本只在一次备份或修复的新状态完整写好后切换，取消、崩溃或写入失败都必须留下上一份可解析版本。对没有账本的旧备份，允许用户显式建立本地基线，但首次建立只能标记为来源未验证，不能把当前字节自动声称为已与空间原件核对。原有的增量跳过、断点续传、失败项重试、备份记录和查看页继续可用，但凡是根据账本声称健康或跳过的媒体，其路径和内容必须与持久记录一致。

**Assistant**

我先了解一下项目结构，再确定如何落地这套完整性账本方案。

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/092113/project-02

这是一个 QQ 空间备份工具（Go）。需求涉及四类备份、下载链路、记录与查看页，属于跨模块的大改动。我先读核心文件了解现有架构。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/cmd/qq-zone/main.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/task_record.go

这是一个跨多个模块的复杂功能（四类备份的账本登记、离线核验、选择性修复、原子切换、旧备份基线、与现有增量/续传/重试/查看页集成）。我先加载规格模式技能来组织需求与实现。

toolName: Skill

status: success

进入 Spec Mode。我先读取工件模板，然后深入勘察四类备份链路、下载层、CLI 菜单和查看页。

toolName: view_files

status: success

filePath: /Users/vance/.trae-cn/builtin/trae/doutops/skills/TRAE-spec-mode/references/artifact-templates.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go

继续勘察 HLS 中间产物、说说/留言板备份链路、CLI 菜单和配置。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

继续看说说/留言板备份的媒体保存路径与失败项结构，以及配置文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/config.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/pkg/util/util.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/group.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/go.mod

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

代码勘察已完成。我已经摸清四类备份的目录布局、下载完成信号（`.resume.json` sidecar、HLS 直写目标文件）、稳定来源身份（相册 sloc、说说/留言 tid+media id）、增量跳过与失败重试链路。在写规格前，有几个影响行为的关键取舍需要确认。

toolName: todo_write

status: success

Todos updated: 5 items

toolName: todo_write

status: success

Todos updated: 5 items

toolName: AskUserQuestion

status: success

三个取舍均按推荐确认。现在编写规格文档。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/spec.md

changes: undefined

规格完成。现在派生实现计划 tasks.md。

toolName: todo_write

status: success

Todos updated: 5 items

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: todo_write

status: success

Todos updated: 5 items

toolName: NotifyUser

status: success

审批通过，进入实现阶段。开始 Task 1：账本核心模型、原子存储与哈希工具。

toolName: todo_write

status: success

Todos updated: 5 items

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

先在 util 中添加流式 SHA-256 摘要工具：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/pkg/util/util.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/pkg/util/util.go

changes: undefined

现在创建账本核心 [ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go)：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

changes: undefined

编写账本单元测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

changes: undefined

toolName: Shell

status: success

测试应用 `errors.Is` 判断包装错误：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

changes: undefined

toolName: Shell

status: success

Task 1 完成。开始 Task 2：HLS 临时文件 + 原子改名。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

changes: undefined

编写 HLS 测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

changes: undefined

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

changes: undefined

toolName: Shell

status: success

Task 2 完成。开始 Task 3：相册/群相册 Spider 集成账本。先在 ledger.go 增加通用跳过决策：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

changes: undefined

创建 Spider 与账本的衔接层 [spider_ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go)：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go

changes: undefined

现在改 [spider.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go)：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

编译检查后写 Spider 集成测试：

toolName: Shell

status: success

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_integrity_test.go

changes: undefined

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_integrity_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

Task 3 完成。开始 Task 4：说说/留言板集成。先看 board_backup.go 的 Backup/Retry 开头：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

创建说说/留言板共用的账本会话：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go

changes: undefined

集成说说备份：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

changes: undefined

同样集成留言板：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

changes: undefined

toolName: Shell

status: success

编写说说/留言板账本集成测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

changes: undefined

toolName: Shell

status: success

说说的跳过计数体现在 ImageCount（Skipped 是帖子级），修正断言：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

changes: undefined

toolName: Shell

status: success

Task 4 完成。开始 Task 5：离线核验器。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go

changes: undefined

编写核验器测试（全类别夹具 + 稳定性 + 只读快照）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

清理测试文件里的占位垃圾：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: Shell

status: success

Task 5 完成。开始 Task 6：基线与纳入未登记。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go

changes: undefined

基线测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline_test.go

changes: undefined

toolName: Shell

status: success

Task 6 完成。开始 Task 7：修复计划与定点修复编排（最大的一块）。先加任务模式常量与 Spider 白名单支持相册 ID：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/task_record.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

changes: undefined

创建修复计划与编排核心 [integrity_repair.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go)：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

创建生产环境在线修复执行器：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go

changes: undefined

toolName: Shell

status: success

编写修复编排测试（假执行器模拟链路）：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

changes: undefined

toolName: Shell

status: success

发现真实语义问题：同一相对路径上已验证条目更换身份时（修复重建、身份迁移）应替换旧条目，而不是并存。修正 `Upsert`：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

修复 vet 警告：会话含锁，应返回指针：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

changes: undefined

toolName: Shell

status: success

Task 7 完成。开始 Task 8：免登录 CLI 菜单。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

changes: undefined

接入主菜单：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

changes: undefined

toolName: Shell

status: success

Task 8 完成。进入 Task 9：全量回归与冒烟验证。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: Shell

status: success

全量绿。现在做端到端冒烟（真实 storage 布局 + 大文件流式性能观测），用临时测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/smoke_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/smoke_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/smoke_integrity_test.go

changes: undefined

toolName: Shell

status: success

冒烟通过：四类域根全链路正常；1GiB 摘要 643ms、约 1.6 GiB/s、Alloc 仅 3.2MiB（流式验证）。删除临时冒烟测试并跑最终回归：

toolName: delete_file

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

实现队列全部完成，进入 Review 阶段。先建立审查检查点，再委派一次全新上下文的只读独立审查。

toolName: todo_write

status: success

Todos updated: 5 items

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/review.md

changes: undefined

现在委派全新上下文的独立只读审查：

toolName: no_need_execute

status: no_need_execute

toolName: todo_write

status: success

Todos updated: 6 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/spec.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/review.md

规格已读完。现在读核心实现文件。

toolName: todo_write

status: success

Todos updated: 6 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

继续读 glue 与核验/基线/修复文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go

现在读 hls.go、http.go、util.go 与 task_record.go。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/pkg/util/util.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/task_record.go

核心实现已读完。现在读测试文件评估覆盖真实性。

toolName: todo_write

status: success

Todos updated: 6 items

toolName: search_by_regex

status: success

toolName: file_search

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_integrity_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

测试已读完。现在运行构建、vet、测试与 Windows 交叉编译。

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

构建全绿。再用 -race 跑一遍相关包，验证并发安全声明。

toolName: Shell

status: success

全绿且 -race 无异常。补充几项走查：viewer 是否引用 .integrity、go.mod 版本、以及几个边界。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/go.mod

构建测试全绿。但走查发现一个高风险疑点：`Ledger.Save()` 会对 `l.Entries` 原地排序却不重建下标。我在仓库外用一个临时 go module（只读引用仓库源码）实证它是否会破坏后续 `SkipDecision`。

toolName: Shell

status: success

internal 限制。改为把仓库复制到 /tmp（只读原件不动），在副本内执行实证。

toolName: Shell

status: success

高严重度缺陷已实证。再快速实证第二个疑点：说说/留言板域根中 `avatars/` 下的 sidecar 是否会被误报为「未完成」。

toolName: Shell

status: success

确认。再延伸实证高严重度索引问题的另一面：索引错乱后 `Upsert` 的「同 rel 接管」会不会误删其它条目。

toolName: Shell

status: success

两个后果都实证。清理临时副本，再跑一遍新增测试的详细输出作为证据。

toolName: delete_file

status: success

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: Shell

status: success

所有走查与取证完成。按内部要求先调用 handoff skill。

toolName: todo_write

status: success

Todos updated: 6 items

toolName: Skill

status: success

toolName: todo_write

status: success

Todos updated: 6 items

Summary:

# 独立审查报告：媒体完整性账本、离线核验与定点修复

审查方式：通读 spec/tasks/review、全部新增与修改文件、7 个新增测试文件；在仓库根实际运行 build/vet/test/交叉编译/-race；对两个静态疑点在 /tmp 的仓库副本中写了一次性验证程序实证（验证后已删除副本，仓库本身零改动）。

**总体结论：fail。** 发现 1 个 high 可操作缺陷（F1，账本内存下标在 `Save()` 后错乱，打穿 FR-4 跳过闸门并可误删健康条目，已实证）、2 个 low 可操作缺陷（F2 管理范围外误报、F3 修复缺二次确认）；另有 6 条 advisory。

---

## 检查点逐项结论

### CP-R1 四类账本登记结构正确且与磁盘一致 —— **fail**
- 身份格式、字段、域根、rel/size/sha 计算本身正确：[ledger.go#L123-L155](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L123-L155)、[EntryFromFile ledger.go#L356-L373](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L356-L373)；`TestLedgerSourceIDFormats`、`TestSpiderStageAndCommitAlbum`、`TestMediaLedgerSessionDecisionAndStage` 均真实断言并通过。
- 但 F1 使得**多相册备份后账本可丢失健康条目、登记与磁盘不再一致**（见下 F1，已实证 b0 条目从落盘账本消失）。单相册/说说/留言板单提交路径正确，端到端不成立，故 fail。

### CP-R2 仅完成态登记 —— **pass**
- 相册：仅 `downloadErr==nil` 且 `!isSkip` 才 stage（[spider.go#L626-L659](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L626-L659)，失败在 L631 `continue`）；mood/board 同样失败即 return 不 stage（[mood_backup.go#L786-L815](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go#L786-L815)、[board_backup.go#L570-L599](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go#L570-L599)）。
- HLS 严格 `<target>.part` → Sync → Close → `os.Rename`，5 个失败返回点（ctx 取消、分片错误、状态码、写失败、Sync 失败）全部保留 .part 且不产生最终文件名（[hls.go#L102-L207](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go#L102-L207)）；`TestDownloadHLSKeepsOnlyPartOnFailure` 实测失败时最终文件不存在、.part 非空。
- 直链 sidecar 成功后删除（[http.go#L531-L532](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go#L531-L532)）。跳过项（含无条目旧 HEAD/非空跳过）不写账本，有测试 `TestMoodDownloadOneMediaLedgerSkipBranches`（nil client 证明零网络）。

### CP-R3 原子切换 / bak 回退 / 取消保留上一版 —— **pass**
- tmp（固定名 path+".tmp"）+ Write + Sync + Close + rename（[ledger.go#L471-L500](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L471-L500)）；json 坏回退 bak、二者皆损 `ErrLedgerCorrupt`、json 缺失但 bak 在可恢复（[ledger.go#L425-L451](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L425-L451)），三条均有测试。
- 取消不提交：相册 `commitAlbumLedger(..., clean=false)` 丢弃 staged（[spider_ledger.go#L100-L123](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go#L100-L123)，字节不变测试）、RetryFailed 取消 L263、mood/board `commit(ctx.Err()==nil)`（[mood_backup.go#L170](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go#L170)、[board_backup.go#L191](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go#L191)）。
- 并发：worker 只在 mutex 下 append staged，提交均在 `g.Wait()/wg.Wait()` 之后，提交点串行；`go test -race ./internal/app/... ./internal/net/http/...` 通过。
- 小瑕疵见 advisory A1/A2（bak 写失败被吞、无父目录 fsync），不破坏原子语义。

### CP-R4 账本一致性闸门 —— **fail**
- 闸门函数本身正确：stat 常规文件、size>0、size、sha256 四核对（[ledger.go#L332-L352](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L332-L352)），四分支测试通过；mismatch 后直链 `Download` 会因无 sidecar 删文件重下（[http.go#L277-L282](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go#L277-L282)），不会被续传拼接污染。
- 但 F1 导致多相册场景下闸门查到的是**其他条目的 rel/sha**：实证缺失文件被判 healthy 离线跳过。无条目分支不写健康声明是对的（[spider.go#L605-L610](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L605-L610)），但整体闸门不可靠，fail。

### CP-R5 离线核验四分类、零网络、零修改 —— **fail**
- 零网络/零修改成立：[integrity_verify.go#L6-L13](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go#L6-L13) 仅 import os/path/sort/strings/util，全链路无 http.Client、无登录态；`TestVerifyAlbumFixtureAllCategories` 断言两次结果 reflect.DeepEqual 且整树 sha256 快照一致。分类优先级、稳定排序、size/hash 两子类型、sidecar/孤立 part 映射均有实测。
- 但 F2：说说/留言板对管理范围外的标记误报「未完成」（已实证），故 fail。

### CP-R6 基线显式、来源未验证、未登记不自动修复 —— **pass**
- 无账本+二次确认才建（CLI [integrity.go#L143-L147](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L143-L147)）；已有账本 `ErrLedgerExists`；未完成排除单列；local 身份 + `origin_verified=false`（[integrity_baseline.go#L29-L85](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go#L29-L85)）；AdoptUnregistered 幂等、不动媒体（测试断言仅 ledger.json/bak 变化）；未登记不入修复计划（[integrity_repair.go#L150-L154](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L150-L154)，测试断言）。

### CP-R7 定点修复隔离 / 健康不重写 / 跨运行续作 —— **pass（带备注）**
- 隔离严格限定计划内 rel + marker + 两个标准标记名，有 `..` 前缀拦截（[integrity_repair.go#L459-L493](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L459-L493)）；missing 不隔离；健康/未登记 mtime 实测不变；隔离保留原相对结构（测试断言 sidecar 进 quarantine）。
- 完全复用原链路：[integrity_repair_runner.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go) 只调 Spider.Download（增量+相册名/ID 白名单）与 Mood/BoardBackup.RetryFailed；FailedItem 契约字段（Kind/Name/TargetUin/MoodTID/MediaID/MediaURL）与 RetryFailed 消费一致（对照 [mood_backup.go#L222-L313](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go#L222-L313)）。
- 链路级异常（断网/无凭证/取消）时不回写计划、原样保留（L379-L398 提前 return，仅在 quarantine 之后；登录在 ExecuteRepair 之外，FR-8「登录失败不动文件」成立）；成功项移除、失败保留 last_error、空计划自删，均有测试。裁决 `ledgerRelHealthy` 用**重新 LoadLedger** 的账本+磁盘 size/sha（L402-L415），严谨。
- 备注：F1 也会影响多相册修复内部 spider 的跳过判断，但裁决端以重加载账本+磁盘为准，后果是「未重建项留在计划」偏保守，不会误报 resolved。另见 A3（扩展名变化致计划项永久残留）、A6（修复 FailedItem 未带 IsVideo）。

### CP-R8 菜单免登录 / 计划先于登录 / 破坏动作确认 —— **pass（带 F3）**
- [menu.go#L267-L269](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go#L267-L269) 明确不 ensureLogin；[integrity.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go) 全文件无登录/网络调用；`plan.Save()`（L241）严格先于 `ensureLogin`（L249），登录失败明确提示计划保留。
- 菜单重排后 15 个选项与 Description case 0-14 逐一核对，无错位（完整性入口为 index 9，描述同步）。
- 基线（L144）、丢弃计划（L299）有 Confirm；**隔离修复没有独立 Confirm**（F3）。考虑到菜单项本身是显式选择、文案明示隔离+登录、且其后还有登录闸，判 pass 但列 low 缺陷。

### CP-R9 构建/vet/测试与回归 —— **pass**
- 实测（仓库根）：`go build ./...` → BUILD_OK；`go vet ./...` → VET_OK；`go test ./... -count=1` → internal/app、internal/net/http、internal/qzone 全部 ok；`GOOS=windows go build ./...` → WIN_BUILD_OK；`gofmt -l .` 无输出；`go test -race ./internal/app/... ./internal/net/http/...` → ok。
- 非增量 `RemoveAll` 只删相册子目录，域根 `.integrity/` 在上级，有 `TestFullBackupWipeDoesNotTouchDomainLedger`；viewer 包 grep 无任何 integrity/ledger 引用；既有 board/mood/spider 测试未被弱化（断言真实，未见放水）。

### CP-U1 集成一致性与复用度 —— **4/5**
四类对称接入、身份/决策/暂存/提交抽象统一、零平行下载实现、修复任务记录并入既有 TaskRecord（`TaskModeRepair`）。扣分项：F1 属于集成边界缺陷；mood 与 board 的决策点各写一份（glue 已共享大部分）、少量不对称。

### CP-U2 终端报告可用性 —— **4/5**
四段分明（缺失/变更/未完成/未登记）、健康区分已验证/未验证、表格风格与现有 CLI 一致、有问题合计与不可修复项风险提示。扣分：变更段不展示期望/实际摘要差异（只展示大小），不可修复项仅在修复结束后列出。

### CP-U3 大文件流式 / 内存 / 进度 —— **4/5**
`FileDigest` 固定 256KiB `io.CopyBuffer` 一次扫描得 size+sha（[util.go#L39-L64](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/pkg/util/util.go#L39-L64)），不随文件体积增长；CLI 核验带 mpb 文件级进度（[integrity.go#L168-L192](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L168-L192)）。本地跳过核对需顺序读全文件属规格接受的 I/O。

---

## Actionable 发现

### F1（high）`Ledger.Save()` 原地排序后不重建索引，多相册增量备份跳过闸门失效并可误删健康条目
- **位置**：[ledger.go#L385-L390](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L385-L390)（Save 内 `sort.SliceStable(l.Entries,...)`）；受影响读路径 [FindBySource ledger.go#L218-L227](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L218-L227)、[Upsert 接管逻辑 ledger.go#L253-L272](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L253-L272)；调用点 [spider.go#L487](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L487) 与 [spider.go#L599](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L599)。
- **机理**：Save 把 `l.Entries` 原地重排，但 `bySource/byRelPath` 仍保存排序前的下标，且之后不调用 `rebuildIndex()`。Spider 每跑完一个相册 Save 一次，后续相册的所有 `SkipDecision`/`Upsert` 都通过错位下标访问条目。
- **复现（已在 /tmp 仓库副本实跑，输出为证）**：① 账本含 A 相册 2 文件、B 相册 2 文件并落盘；② 二次运行 LoadLedger 后删除 `B/b0.jpg`；③ A 相册增量新增 `a2.jpg` 并走到相册边界 Save；④ 对 B0 调 `SkipDecision`。实测：
  - 提交前 `B0 decision=2 (mismatch)`，提交后 `B0 decision=1 (healthy)`——**缺失文件被离线跳过、不再重下**，直接违反 FR-4/AC-5；
  - `FindBySource(B0)` 返回 `rel="A/a2.jpg"`，`FindByRelPath("B/b0.jpg")` 返回 A2 的 source——下标全面串号；
  - 随后模拟 B1 损坏重下成功并 `Upsert`，再次落盘后 `B/b0.jpg` 条目从账本中被**误删**（接管逻辑通过错位 byRelPath 删掉无关健康条目），AC-1/AC-4 的「每个成功媒体恰有一条」被破坏。
- **触发条件（常见路径，非极端）**：≥2 个相册的个人/群相册增量备份，且第一个提交的相册有至少 1 个新下载条目（首次全量备份在第二相册起也会触发，因为首批 Save 前内存条目是下载完成顺序而非排序顺序）。多相册定点修复内部 spider 同样受影响。mood/board 与 RetryFailed 因进程内只提交一次且之后不读账本，不受影响；磁盘上的 JSON 始终有序、重新 Load 即恢复，故不表现为文件损坏。
- **期望行为**：Save 后内存索引与 Entries 顺序保持一致；任何后续 SkipDecision 必须命中本 source 的条目。
- **建议修复方向**：Save 排序后立即 `l.rebuildIndex()`（或对副本排序/marshal 后重建），并补「相册 A Save 后再对相册 B 的 source 做 SkipDecision/Upsert」的回归测试。

### F2（low）说说/留言板核验把管理范围外（avatars 等）的残留标记误报为「未完成」
- **位置**：[integrity_verify.go#L272-L284](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go#L272-L284)：marker 判定（L273-L280）在 `inManagedScope` 过滤（L282）之前，且 `scanManagedFiles` 对整个域根 Walk。
- **复现（已实跑）**：shuoshuo 域根放 `media/2020/01/a.jpg` 与 `avatars/10001.jpg` + `avatars/10001.jpg.resume.json`（头像下载中断的真实遗留），`ScanRootWithoutLedger` 输出 `status=incomplete rel=avatars/10001.jpg.resume.json`。违反 FR-5「说说/留言板仅扫描 media/ 目录」与 Non-Goals（avatars 不报告）；该误报还会进入 repair.json（被标 `auto_repairable=false`，不会隔离文件，但永久污染计划与四段统计）。
- **期望行为**：mood/board 仅当 marker 目标位于 `media/` 前缀内才计入未完成。
- **建议修复方向**：marker 入列前同样过 `inManagedScope(kind, mk.targetRel, …)`；补 avatars sidecar 不产生噪声的用例。

### F3（low）隔离修复缺少独立二次确认
- **位置**：[integrity.go#L211-L217](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L211-L217) → [runRepair L226-L253](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L226-L253)：选择「🔧 修复以上问题」后直接 plan.Save→登录，无 `askYesNo`；而基线（L144）、丢弃计划（L299）均有确认。
- **依据**：FR-9 要求「所有破坏性或外发动作（建立基线、隔离修复、丢弃计划）都必须有显式确认」；tasks.md TR-8.3 自称「基线/修复/丢弃均有 Confirm」，与实现不符。当前仅有菜单选项文案与后续登录两道闸。
- **建议方向**：runRepair 落计划前增加一次 Confirm，文案明示「将移动损坏文件到 quarantine、需要登录、健康文件不重写」。

## Advisory（不阻塞）
- **A1**：[ledger.go#L411-L413](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L411-L413) 静默吞掉 ledger.bak 写失败（返回 nil），bak 可能长期停留更旧版本；建议至少告警。
- **A2**：`atomicWriteFile` 使用固定 `.tmp` 名且 rename 前未 fsync 父目录；单进程假设下可接受（规格已声明不加文件锁、接受 POSIX rename 原子性），记录在案。
- **A3**：修复裁决只按 `rel_path` 健康；若重建时 Content-Type 改扩展名（[http.go#L350-L369](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go#L350-L369)），旧 rel 永不 healthy，计划项永久残留（不损坏文件）。建议按 source_id 裁决或迁移计划项 rel。
- **A4**：三处账本提交失败仅 Warn 并提示「下次备份会补齐登记」（[spider_ledger.go#L120-L122](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go#L120-L122) 等），但无条目旧文件在增量模式走旧跳过（HEAD/非空）并不会补登记，只能靠用户纳入基线；文案与实际不符。
- **A5 测试盲区**：① F1 场景（跨相册 Save 后再决策）无任何测试；② HLS 仅测分片 500，TR-2.2 声称的 ctx 取消路径未测；③ 核验对「范围外 marker」无测试（F2 因此漏网）；④ CLI 交互按任务说明免测，已通过代码走查替代。说明：Windows 替换式 rename 未做运行时测试，但 Go `os.Rename` 在 Windows 走 `MoveFileEx(MOVEFILE_REPLACE_EXISTING)`，hls.go:205 覆盖已存在目标的行为可成立，未计为缺陷。
- **A6**：[moodFailedItems integrity_repair.go#L442-L456](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L442-L456) 未设置 `IsVideo`；当 backup.json 中找不到 media 但失败项确为视频时，getinfo 换源链路不会触发，降低兜底成功率。

Verification:
- `go build ./...` → BUILD_OK；`go vet ./...` → VET_OK。
- `go test ./... -count=1` → 全部包 ok（internal/app 0.890s、internal/net/http 0.738s、internal/qzone 1.039s），无失败/跳过。
- `GOOS=windows go build ./...` → WIN_BUILD_OK；`gofmt -l .` 无输出。
- `go test -race ./internal/app/... ./internal/net/http/... -count=1` → ok（账本并发边界无数据竞争）。
- 新增 29 个相关用例 `-v` 运行全部 PASS（含账本原子/回退/双损坏、四分支跳过、取消字节不变、四类别夹具、两次报告 DeepEqual、整树快照不变、隔离 mtime、断网计划保留、二轮续作、HLS 成功 rename/失败只留 .part）。
- F1/F2 均通过在 /tmp 的仓库副本中一次性 Go 程序实跑取证（副本已删除，未改动仓库任何文件；审查全程只读）。

Gaps & Unverified:
- CLI 交互（survey 菜单流）未做手工运行，结论基于代码走查：免登录、无网络 import、计划先于登录均已证实；实际终端 UI 观感（AC-14）仅静态评估给 4 分。
- 未做真实 GB 级文件计时（NFR 证据来自 tasks.md 自述与代码层固定缓冲走查，未独立复测）。
- 真实网络链路（修复登录、群授权、视频换源）按要求未联网验证，复用结论基于调用链走查与契约比对。
- 仓库未提交任何修改（git status 仅原有的 M/?? 工件），review.md 也未按要求改动（只读审查，证据在本报告内）。

Artifacts:
- 关键文件：[ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go)、[spider_ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go)、[spider.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go)、[media_ledger_glue.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go)、[mood_backup.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go)、[board_backup.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/board_backup.go)、[integrity_verify.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go)、[integrity_baseline.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go)、[integrity_repair.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go)、[integrity_repair_runner.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go)、[internal/cli/integrity.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go)、[hls.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go)、[http.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go)。

Next Action:
- 总体判定 **fail**：需先修复 F1（Save 后 rebuildIndex + 跨相册回归测试），建议顺带修 F2、F3，再重新跑全套构建/测试与本审查；F1 修复前，多相册增量备份「依据账本跳过」的结果不可信，不建议发布。

独立审查结论为 **fail**：1 个 high（Save 排序后索引失效，多相册场景打穿跳过闸门）+ 2 个 low（范围外 marker 误报、修复缺二次确认）+ 若干 advisory。先把结果记入 review.md，并在 tasks.md 物化修复问题。

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/review.md

changes: undefined

在 tasks.md 物化审查问题：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

修复 I-1：Save 排序后重建索引。

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

changes: undefined

补 F1 回归测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

changes: undefined

修复 I-2：marker 也要过管理范围过滤：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go

changes: undefined

补 F2 测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

修复 I-3：修复前独立确认：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

changes: undefined

修复 I-4（advisory 三项 + 取消测试）。先 A3 裁决与 A6 IsVideo：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go

changes: undefined

补 HLS ctx 取消测试：

toolName: edit_file_search_replace

status: failed

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

changes: undefined

toolName: Shell

status: success

toolName: Shell

status: success

补 A3/A6 修复测试：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go

changes: undefined

toolName: Shell

status: success

测试断言有误：无账本扫描时 `media/a.jpg` 本就该报未登记，只有「范围外标记」应为 0。修正：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

changes: undefined

toolName: Shell

status: success

全部修复通过（含 race）。更新问题状态并启动 R2 全新审查：

toolName: Shell

status: success

toolName: Shell

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

changes: undefined

toolName: no_need_execute

status: no_need_execute

toolName: todo_write

status: success

Todos updated: 9 items

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/092113/project-02

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/review.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/spec.md

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/tasks.md

现在阅读核心代码文件。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

`Save` 的 rebuildIndex 已在 385-393 行确认。注意到 `Upsert` 更新分支（259-263 行）在同 source 但 rel_path 变更时，旧 rel_path 的 byRelPath 映射可能残留。继续读其余代码确认可达性。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go

F2 修复已确认（275 行 scope 先于 marker 识别）。继续读修复编排。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go

继续读修复 runner、CLI 入口与 HLS。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

F3 两条路径已确认。现在读 HLS 与 mood 重试链路，确认 IsVideo 是否真的影响 getinfo 换源。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go

toolName: search_by_regex

status: success

A6 链路已确认（isVideo 影响 937/942/993 行的换源与 HLS 兜底）。现在读新增/相关测试。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls_integrity_test.go

现在看 spider 与 mood 的 ledger 集成层，确认 staged 提交与 Upsert 的相对顺序。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go

检查是否还有其它直接改动 Entries 切片的位置。

toolName: search_by_regex

status: success

继续读核验测试与修复测试其余部分。

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go

现在检查 spider.go 中账本闸门的实际调用点。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go

代码走查大体完成。更新进度，开始全量构建/测试，同时准备 /tmp 独立复核。

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

toolName: Shell

status: success

全量绿。现在在 /tmp 副本中编写独立场景复核 F1（不改动原仓库任何文件）。

toolName: Shell

status: success

toolName: Write

status: success

filePath: /tmp/r2check/internal/app/r2_independent_check_test.go

changes: undefined

修正测试中的一处括号错误。

toolName: edit_file_search_replace

status: success

filePath: /tmp/r2check/internal/app/r2_independent_check_test.go

changes: undefined

toolName: Shell

status: success

测试自身的期望写错（第二轮加载时本就应是 7 条）。修正后复跑。

toolName: edit_file_search_replace

status: success

filePath: /tmp/r2check/internal/app/r2_independent_check_test.go

changes: undefined

toolName: Shell

status: success

F1 在更强的三相册交错场景下独立复核通过。现在穷尽 FindByRelPath 的消费者，评估改名窗口陈旧映射的可达性。

toolName: search_by_regex

status: success

生产消费者只有修复裁决（用的是重新 Load 的账本）与纳入基线（仅 local 新 rel），改名窗口不可达。再快速核对 board 链路与菜单接线。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

检查 HLS 目标已存在时的 Windows 替换处理（tasks.md 声称做了兼容）。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/http.go

再显式运行修复说明中点名的新增/相关测试，留存真实证据。

toolName: Shell

status: success

toolName: Shell

status: success

最后扫一遍群相册构造签名与取消提交相关测试名，确认接线无误。

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

所有验证完成。清理 /tmp 副本并确认 go.mod 版本声明，然后准备交接。

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 9 items

toolName: Skill

status: success

# R2 独立代码审查报告：媒体完整性账本 / 离线核验 / 定点修复

## 总体结论：**pass**

R1 的 F1/F2/F3 三项 actionable 问题均已真实修复，I-4 的四项 advisory 处理（A3/A4/A6/A5 测试盲区）经验证有效。R2 在全新上下文通读全部代码与测试、实跑全套命令，并在仓库**零改动**前提下于 /tmp 副本设计了不同于官方用例的三相册交错场景独立复核 F1，未发现新的 actionable 缺陷；仅有 4 条不阻塞的 advisory。

- 审查环境：darwin/arm64，实际工具链 `go1.26.1`（仓库声明 `go 1.25.0`）
- 原仓库未做任何修改；独立复核测试仅存在于 /tmp 副本，已清理

---

## 12 个检查点逐项结论

### CP-R1（四类账本结构与磁盘一致）— **pass**
- R1 失败原因 F1 已修复：[ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L385-L393) 在 `sort.SliceStable` 原地排序后立即调用 `l.rebuildIndex()`，bySource/byRelPath 在落盘前重建。
- 全部 `Entries` 原地变更点穷尽排查：[removeAt](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L275-L281)、[RemoveByRelPathPrefix](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L286-L302)、[Remove](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L305-L315) 均重建下标；Upsert append 路径（266-271 行）双映射维护正确；全仓库再无第二处直接改 Entries 的生产代码（grep `.Entries =` 仅 ledger.go 内部）。
- 官方用例 `TestLedgerSaveKeepsIndexesValidAcrossAlbums` 通过；R2 自设场景同样通过（见 Verification）。

### CP-R2（仅完成态登记）— **pass**
- [hls.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go#L105-L207) 分片写 `<target>.part`，5 个返回点（ctx 取消 152-157、分片错误 162-168、状态码 169-175、写错误 177-183、Sync 失败 198-201）均 Close 后返回且不改名；仅 198-207 全段 Sync+Close 后 `os.Rename` 产生最终文件。
- I-4 新增取消路径测试 `TestDownloadHLSContextCancelKeepsOnlyPart`（分片阻塞→cancel，断言最终文件不存在、.part 保留已写字节），实跑通过。Spider/Mood 均仅在下载成功后 stage（[spider.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L655-L659)、[media_ledger_glue.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go#L70-L86)）。

### CP-R3（原子切换 / bak 回退 / 取消保留）— **pass**
- [atomicWriteFile](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L474-L503) 同目录 tmp+Sync+rename；[LoadLedger](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L428-L454) 损坏回退 bak、双损坏 ErrLedgerCorrupt；取消路径 commit(false) 丢弃 staged（[spider_ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider_ledger.go#L100-L123)、[media_ledger_glue.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go#L88-L106)），有测试 `TestSpiderCommitAlbumCanceledKeepsPreviousLedger`、`TestMediaLedgerSessionCancelDiscardsStaged` 覆盖，全套绿。

### CP-R4（账本一致性闸门）— **pass**
- [SkipDecision](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L332-L352) 纯本地 stat+size+SHA-256；四分支测试 `TestLedgerSkipDecisionFourBranches` 通过。
- R2 独立场景中「A 边界提交后删 B0 文件 → B0 mismatch、A0/C0 healthy」「两轮 load/save 后删 C1、新增 A 项不误删 C1/B0」全部断言通过。
- 无条目回退旧 HEAD 逻辑且不登记：[spider.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go#L599-L611)、[mood_backup.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go#L258-L274)。

### CP-R5（离线核验四分类、零网络零修改）— **pass**
- F2 已修复：[scanManagedFiles](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go#L267-L291) 先排除 `.integrity/`，再过 [inManagedScope](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go#L305-L313)，最后才识别 marker；mood/board 仅 `media/` 前缀内的 sidecar/.part 计入未完成。
- R2 在 /tmp 实测：mood 的 `avatars/*.resume.json`、`assets/*.part` 在有账本核验与无账本扫描中均零噪声；`media/` 内两类 marker 仍正确识别；**album 域根**的 `rootghost.mp4.part` 与子目录 sidecar 仍识别（scope 改动不影响相册域）；`.integrity/*` 与 `album_metadata.json` 仍排除。官方 `TestVerifyMoodIgnoresMarkersOutsideMediaScope` 通过。
- 零网络：integrity_verify.go import 仅 os/path/sort/strings/util；零修改有整树快照测试。

### CP-R6（基线显式、未验证、未登记不自动修复）— **pass**
- [EstablishBaseline](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go#L29-L85) 已有账本即 ErrLedgerExists；未完成排除；[AdoptUnregistered](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go#L90-L123) 幂等且不碰媒体；[NewRepairPlan](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L129-L156) 只收 missing/changed/incomplete。测试全绿。

### CP-R7（定点修复范围与计划续作）— **pass**
- 隔离仅限计划内目标+marker（[quarantineItem](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L461-L495)，带 `..` 逃逸防护）；健康/未登记文件 mtime 不变有测试；白名单含相册名+ID；mood/board 复用 RetryFailed。
- A3 修复正确：[ledgerSourceResolved](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L516-L521) 对非 local 身份调 SkipDecision。R2 专门评估了「同 source 在别处健康但计划 rel 仍坏」误判风险——**不可达**：新 rel 进入账本的唯一途径是下载成功后 EntryFromFile+Upsert；修复前条目必指向计划 rel，quarantine 已移走坏文件；runner 失败则条目不更新、旧 rel 文件缺失→mismatch；且裁决前在 [403 行](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L403) 重新 LoadLedger，不读 runner 内存态。`TestRepairResolvedBySourceAfterExtensionRename`（a.jpg→a.png）通过。
- 链路级异常早返回不写计划（379-398 行），续作双轮测试通过。

### CP-R8（菜单免登录、计划先于登录、破坏动作确认）— **pass**
- [menu.go:267-269](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/menu.go#L267-L269) 明确不 ensureLogin。
- F3 已修复，R2 走查两条路径：
  1. **新计划**：[runRepair](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L241-L263) 在 `plan.Save()` 之前独立 `askYesNo`（默认**否**，文案含隔离/登录/健康不重写）；拒绝时直接 return——repair.json 不落盘、不登录；若磁盘上有旧 repair.json 也保持原样不被覆盖。
  2. **继续既有**：[integrity.go:106-109](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go#L104-L110) 的既有询问即确认，`alreadyConfirmed=true` 不重复弹——与 I-3/TR-I-3.1 约定一致。
- 计划落盘（251 行）严格先于 ensureLogin（259 行）；登录失败只提示保留计划，ExecuteRepair 未调用，无任何媒体改动。丢弃计划、纳入基线、建基线均各有独立确认。

### CP-R9（构建/vet/测试/交叉编译）— **pass**
真实输出见 Verification；另 `gofmt -l .` 无差异。

### CP-U1（集成一致性 rubric）— **4/5，pass（阈值 ≥4）**
四类对称接入、零平行下载实现、修复并入 TaskRecord（TaskModeRepair），白名单 Spider / RetryFailed 全部复用既有链路；群相册 ownerUin/groupID 接线（RunAlbumRepair→NewGroupSpider→GroupLedgerRoot）走查一致。未给 5：修复裁决/计划逻辑与备份链路间仍需靠「rel 命名约定」隐式耦合（见 A1）。

### CP-U2（终端报告可用性 rubric）— **4/5，pass**
四段 tablewriter 报告+健康已验证/未验证汇总+风险提示（不可修复项、基线语义）；续作提示与修复后 resolved/remaining/unrepairable 汇总完整。

### CP-U3（大文件流式/内存 rubric）— **4/5，pass**
256KiB 固定缓冲流式 FileDigest（util 包），核验/登记均走同一流式摘要；CLI 带 mpb 文件级进度。R2 未重复 1GiB 实测（沿用 R1 实测数据：643ms/Alloc≈3.2MiB）。

---

## I-4 各子项验证

- **A3（source 裁决）**：见 CP-R7，正确且误判路径不可达。
- **A6（IsVideo 透传）**：[moodFailedItems](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go#L443-L458) 传 `IsVideo: it.MediaKind == MediaKindVideo`；R2 确认它在 RetryFailed 中**确有行为影响**：media 树查不回时 `isVideo` 直接来自 item（[mood_backup.go:241](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/mood_backup.go#L241)），进而控制死链重试策略（937 行）、getinfo 换源 `ResolveTencentVideo`（942 行，需 videoID）与无 cookie HLS 兜底（993-996 行）；测试 410-412 行断言 `IsVideo=true`。
- **A4（提交失败文案）**：[media_ledger_glue.go:104](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/media_ledger_glue.go#L103-L105) 与 spider_ledger.go:121/138 文案已改为「下次重新在线下载时补登记，也可纳入本地基线」，与真实补登记条件一致。
- **A5（测试盲区）**：HLS ctx 取消测试与扩展名改名测试均存在且实跑通过；跨相册 Save 测试存在，R2 另补独立场景。

---

## Advisory（不阻塞，无需本轮修复）

- **A1（low，防御性）**：[Upsert 更新分支 ledger.go:259-263](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L259-L272) 在「同 source、rel_path 变更」时只写新键，不删旧的 byRelPath 键，Save 前旧 rel 会陈旧命中改名后的条目（R2 已实测复现：`FindByRelPath("…/a.jpg")` 返回 RelPath 为 a.png 的条目）。当前**无消费者可观测**：commit 循环中 Upsert 与 Save 间无读操作；ExecuteRepair/AdoptUnregistered 均用新 Load 的账本；触发二次误删需要两个不同 source 映射到同一最终路径，而文件名由 sloc/身份 MD5 派生，不可达。建议在更新分支补 `if old := l.Entries[i].RelPath; old != e.RelPath { delete(l.byRelPath, old) }`，把不变量收紧。
- **A2（low，Windows 边界）**：[hls.go:205](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go#L198-L207) 最终化为裸 `os.Rename(part,target)`；tasks.md Task 2 曾声称「目标已存在时做 Windows 兼容替换」，实际无替换处理。Windows 下若 target 已存在（死链普通下载残留半成品后走 993 行 HLS 兜底），rename 会失败。属本次特性既有代码、R1 未标记、触发面窄（修复流程坏文件已先隔离）。
- **A3（low，规格措辞）**：FR-8 字面要求「用户取消时不做任何文件改动」，而 ExecuteRepair 先隔离后执行；runner 期间取消会留下 quarantine 移动。实际计划原样保留、续作可完整重做、隔离为移动非删除，安全性可接受；建议取消提示文案明示「已隔离文件保留在 .integrity/quarantine/」。
- **A4（low，边界行为）**：空相册/全失败相册到达提交边界或 mood 空跑 commit 时，会落一份 0 条目的 ledger.json（[Save 不拦截空 Entries](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go#L377-L418)，EstablishBaseline 空目录同理）。无害（后续备份正常登记、未登记仍可纳入基线），但菜单不再对该域提示建基线。
- 另：继续既有计划的确认默认值为「是」（integrity.go:106），新计划默认为「否」；与 I-3 设计一致，仅提示关注该不对称。

## Verification（实跑命令与真实结果）

| 命令 | 结果 |
|---|---|
| `go build ./...` | exit 0（BUILD_OK，go1.26.1） |
| `go vet ./...` | exit 0（VET_OK） |
| `go test ./... -count=1` | 全部包 `ok`：internal/app 1.750s、internal/net/http 2.122s、internal/qzone 1.708s |
| `go test -race ./internal/app/... ./internal/net/http/...` | 两包均 `ok`（2.556s / 2.550s），无数据竞争 |
| `GOOS=windows go build ./...` | exit 0（WINDOWS_BUILD_OK） |
| `gofmt -l .` | 无输出（GOFMT_CLEAN） |
| 7 个修复相关用例 + 3 个 HLS 用例 `-v` 单跑 | 10/10 PASS（含 TestLedgerSaveKeepsIndexesValidAcrossAlbums、TestVerifyMoodIgnoresMarkersOutsideMediaScope、TestRepairResolvedBySourceAfterExtensionRename、TestDownloadHLSContextCancelKeepsOnlyPart 等） |
| R2 独立测试（仅 /tmp 副本，已删除） | 三相册 A/B/C 乱序插入、逐边界提交、跨相册删文件 SkipDecision/Upsert 交错、save→load→save 两轮：PASS；同 source 改名窗口陈旧键：按预期复现（已记为 A1）；F2 四类边界 marker：PASS |

## Gaps & Unverified

- 未重复 CP-U3 的 1GiB 大文件实测（沿用 R1 数据，流式代码未变动）。
- CLI survey 交互未做真实终端手工运行；F3 结论基于代码走查（交互层无单测是既有约定 TR-8.x 允许的）。
- Windows 仅做交叉编译，A2 的 rename 行为未在 Windows 实测。
- 原仓库严格零改动；所有自设验证均在 /tmp 副本完成并已清理。

**建议：R2 判定本特性可合入（pass）；A1 建议顺手修（一行 delete），A2-A4 可记入后续。**

R2 判定 **pass**。审查建议的 A1 是一行不变量加固（同来源改名时旧 rel 键残留），顺手修掉并补测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go

changes: undefined

toolName: Shell

status: success

R2 已通过。把 R2 结论与复审后加固补记进 review.md：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/review.md

changes: undefined

toolName: todo_write

status: success

Todos updated: 5 items

功能已全部完成并通过两轮独立审查（R1 发现 1 个高危索引缺陷，修复后 R2 判定 **pass**）。

## 交付内容

**账本（四类备份各一份，随备份目录走）**
- 位置：域根隐藏目录 `.integrity/ledger.json`，如 `storage/qzone/<QQ>/album/.integrity/`、`qun/<群>/.integrity/`、`shuoshuo/.integrity/`、`liuyanban/.integrity/`
- 每条目记录：稳定来源身份（`album:<QQ>:<相册ID>:<sloc>`、`group:...`、`shuoshuo/board:<QQ>:<tid>:<媒体ID>`）、POSIX 相对路径、字节数、SHA-256、`origin_verified` 标志
- 仅在下载链路成功返回后登记；HLS 改为 `<target>.part` 分片写入、Sync 关闭后才原子改名，失败只留 `.part`；续传半成品（`.resume.json`）、失败项永不入账本
- 原子提交：tmp+Sync+rename，保留 ledger.bak，双损坏即中止不用空账本覆盖；相册每完整跑完一个提交一次，取消时半程成果丢弃、旧账本不动

**离线核验（主菜单新增「🧪 完整性核验与修复」，免登录、零网络、零修改）**
- 四段稳定报告：**缺失 / 内容变更（大小或摘要）/ 未登记 / 未完成**，健康项区分「来源已验证/基线未验证」
- 头像、viewer 资源、backup.json、raw、album_metadata.json 不在范围，不制造噪声

**基线与定点修复**
- 无账本旧备份：菜单二次确认后建基线，条目一律 `origin_verified=false`，文案明确「未与空间原件核对」；未登记文件可「纳入基线」
- 修复：计划先落 `.integrity/repair.json` 再请求登录（无凭证/断网/取消均保留，下次入口可继续）；损坏文件先移到 `.integrity/quarantine/<时间戳>/` 保留原结构，再走各类型**原有**链路（白名单 Spider / 说说留言板 RetryFailed / 视频 getinfo 换源）；健康文件经 size+SHA-256 闸门保证不重写；修复写 mode=repair 任务记录

**关键文件**：新增 [ledger.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/ledger.go)、[integrity_verify.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_verify.go)、[integrity_baseline.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_baseline.go)、[integrity_repair.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair.go)、[integrity_repair_runner.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/integrity_repair_runner.go)、[internal/cli/integrity.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/cli/integrity.go)；改动 [spider.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/app/spider.go)、mood/board_backup.go、[hls.go](file:///Users/vance/project/mineProject/swe/092113/project-02/internal/net/http/hls.go)、menu.go 等。

**验证**：`go build/vet/test`、`-race`、Windows/macOS 交叉编译全绿；新增约 35 个单测；实测 1GiB 摘要 643ms、约 1.6 GiB/s、常驻 Alloc≈3.2MiB。过程文档在 [.trae/specs/media-integrity-ledger/](file:///Users/vance/project/mineProject/swe/092113/project-02/.trae/specs/media-integrity-ledger/)（spec.md、tasks.md、review.md）。未做 git commit；两条不阻塞的小建议（Windows 下 HLS rename 极端边界、空域根落空账本）已记录在 review.md，可后续处理。
