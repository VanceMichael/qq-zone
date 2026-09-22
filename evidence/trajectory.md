# 只在确认失效时清理历史登录凭证

> **Workspace:** /Users/vance/project/mineProject/swe/092113/project-01

---

**User**

用户从历史账号列表选中一个仍然有效的账号时，一次取消、超时或暂时的接口故障不应让该账号从本地记录中消失，更不应直接弹出新的登录二维码。请修正历史会话的校验和恢复流程：只有 QQ 空间明确返回登录凭证失效时才删除当前选中的账号并引导其重新扫码；context 取消、网络失败、限流、5xx、非预期响应或解析失败都必须保留原凭证，返回可观察错误并让用户稍后重试或重选账号。会话文件损坏或不可读时应停止账号选择和新会话写入，明确报告存储问题，不得把错误当成空账号库后覆盖原文件。校验成功后应保留该账号的群授权和其他账号的全部内容，只在持久化成功后刷新最近使用时间；若保存失败，必须保留旧文件并报错，不得伪装成已完成登录。首次使用时的扫码登录、有效历史账号恢复、多账号切换和用户主动清理账号的原有结果保持不变。

**Assistant**

toolName: view_folder

status: success

/Users/vance/project/mineProject/swe/092113/project-01

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/qzone.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/qzone_login_test.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/net/http/http.go

现在我理解了项目结构。让我查看其余相关调用点，确认会话函数在哪里被使用。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/cmd/qq-zone/main.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/mood.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/group.go

toolName: search_by_regex

status: success

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/group.go

toolName: search_by_regex

status: success

代码库里已有"登录失效"业务码（-3000、-4001、-87998）。让我查看相关测试，避免破坏既有错误信息约定。

toolName: search_by_regex

status: success

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/board.go

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/group.go

我已经掌握了完整上下文。现在制定修复计划：

toolName: todo_write

status: success

Todos updated: 4 items

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/go.mod

现在开始修改 `session.go`：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session.go

changes: undefined

现在修改 `client.go`，加入错误分类和新的校验恢复流程：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

现在添加错误类型、校验方法和分类器：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

现在改造 `GetAlbumList` 的错误返回，携带分类所需的结构化信息：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

错误类型的分支判断需要更明确，我用 kind 字段重构，避免非 200 但有 body 时误入解析分支：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

toolName: todo_write

status: success

Todos updated: 4 items

现在修改 `menu.go` 的 `ensureLogin`：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

changes: undefined

更新 `cli/group.go` 中 `ClearQunAuth` 的调用（签名已变为返回 error）：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/group.go

changes: undefined

先编译检查：

toolName: Shell

status: success

编译通过。优化分类器在 context 取消时保留错误链，然后补测试：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

changes: undefined

现在补充单元测试：

toolName: Write

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go

changes: undefined

修正测试里 SessionPath 恢复值的小问题：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go

changes: undefined

toolName: Shell

status: success

两个问题：`SessionStorageError` 需要实现 `Is` 才能被 `errors.Is` 识别；时间比较要按秒截断。先修：

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session.go

changes: undefined

toolName: edit_file_search_replace

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go

changes: undefined

toolName: Shell

status: success

全部测试通过。让我最终复查修改后的关键代码：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go

toolName: Shell

status: success

全部通过。运行完整测试套件做最终回归确认：

toolName: Shell

status: success

toolName: todo_write

status: success

Todos updated: 4 items

最后通读一遍 `ensureLogin` 确认逻辑完整：

toolName: view_files

status: success

filePath: /Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go

修复完成，全部测试通过（`go build`、`go vet`、`go test ./...` 均干净）。改动涉及 4 个文件：

## 问题根因

- [client.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go#L58-L88) 旧逻辑里 `GetAlbumList` 返回**任何**错误都会 `RemoveSession` 删账号；[menu.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go#L292-L300) 又忽略 `LoadSessions` 错误，损坏文件被当成空库直接弹二维码，随后的保存还会覆盖原文件。
- 旧 `SaveSession` 用 `sessions, _ := LoadSessions()` 吞掉存储错误，且非原子写入、保存失败也伪装成功。

## 修正内容

**1. 存储层加固 — [session.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session.go)**
- 新增 `ErrSessionStorage`/`SessionStorageError` 与 `IsSessionStorageError`；文件不存在仍按空库（首次使用流程不变），但不可读、JSON 损坏、null 条目一律显式报错。
- `SaveSession`/`RemoveSession`/`TouchSession` 遇到损坏文件直接失败，**绝不覆盖**；写入改为临时文件 + fsync + rename 原子替换并保留原权限，任何失败旧文件原样保留。
- 新增 [TouchSession](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session.go#L119-L141)：校验成功后只刷新该账号 `LastUsed`，其 `QunCookie`（群授权）和其他账号全部内容原样保留。

**2. 校验分类 — [client.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go#L189-L317)**
- `CheckLogin` + 分类器：只有空间**明确**返回凭证失效才产出 `CredentialInvalidError`（业务码 -3000/-4001/-87998，与 mood/board 既有判定一致；明确登录文案；或跳转到 ptlogin 登录页 HTML）。
- context 取消/超时、网络失败、429 限流、5xx、其他 4xx、非预期响应、JSONP 解析失败全部归为 `TemporaryLoginError`，默认从严保留凭证；风控 HTML 不会误判失效。
- `GetAlbumList` 错误改为携带 HTTP 码/业务码/原始响应体的 `albumAPIError`，错误文案与历史保持一致。
- [NewClientWithSession](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/client.go#L58-L88)：仅明确失效才删当前账号（删除失败也停止）；暂时故障原样返回；持久化失败报错，不再伪装登录完成。扫码登录 `NewClientWithQR` 落盘失败同样报错。

**3. 交互层 — [menu.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/cli/menu.go#L292-L380)**
- 存储故障：停止账号选择、明确报告路径与原因，不弹二维码；明确失效：提示后才重新扫码；取消/暂时故障：打印可观察原因后返回主菜单，可稍后重试或重选。首次扫码、有效账号恢复、多账号切换、用户主动清空账号的原有结果保持不变。

**4. 测试 — [session_test.go](file:///Users/vance/project/mineProject/swe/092113/project-01/internal/qzone/session_test.go)** 新增 13 个用例：损坏文件不被覆盖、null 条目、目录不可读、原子合并不留临时文件、Touch 只动时间且群授权/他人账号不变、首次使用，以及分类器全矩阵（含取消 context 错误链可观察、风控 HTML 不误判）。
