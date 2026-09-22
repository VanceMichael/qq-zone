# 媒体完整性账本、离线核验与定点修复 - Independent Review

- [x] CP-R1: 四类备份账本登记结构正确且与磁盘一致
  - **Type**: `rule`
  - **Covers**: AC-1, AC-4
  - **Evidence**: R1 fail（F1：多相册 Save 后索引串号，可丢失健康条目）→ 修复后由 R2 复核

- [x] CP-R2: 仅完成态登记（HLS .part/续传 sidecar/失败项不得健康）
  - **Type**: `rule`
  - **Covers**: AC-2
  - **Evidence**: R1 pass：hls.go 5 个失败点均只留 .part；stage 仅在成功路径；TestDownloadHLS* 实测

- [x] CP-R3: 账本原子切换、bak 回退、取消保留上一版
  - **Type**: `rule`
  - **Covers**: AC-3
  - **Evidence**: R1 pass：atomicWriteFile tmp+Sync+rename；损坏回退/双损坏中止有测试；取消 commit(false) 字节不变；-race 通过

- [x] CP-R4: 账本一致性闸门：路径+字节+摘要一致才跳过/声称健康，不一致强制重下
  - **Type**: `rule`
  - **Covers**: AC-5
  - **Evidence**: R1 fail（F1 使多相册场景查错条目）→ I-1 修复后由 R2 复核

- [x] CP-R5: 离线核验四分类稳定且零网络、零修改
  - **Type**: `rule`
  - **Covers**: AC-6
  - **Evidence**: R1 fail（F2：avatars sidecar 误报未完成）→ I-2 修复后由 R2 复核；零网络/零修改本身成立

- [x] CP-R6: 基线显式建立、来源未验证；未登记纳入不自动修复
  - **Type**: `rule`
  - **Covers**: AC-7, AC-12
  - **Evidence**: R1 pass：ErrLedgerExists、二次确认、幂等纳入、未登记不入计划均有测试

- [x] CP-R7: 定点修复只处理确认项（隔离、健康不重写），计划跨运行续作
  - **Type**: `rule`
  - **Covers**: AC-8, AC-9
  - **Evidence**: R1 pass（备注：裁决偏保守，扩展名变更见 advisory A3，I-4 顺手修复）

- [x] CP-R8: 菜单入口免登录；计划落盘先于登录；破坏性动作有确认
  - **Type**: `rule`
  - **Covers**: AC-10
  - **Evidence**: R1 fail（F3：修复无独立确认）→ I-3 修复后由 R2 复核

- [x] CP-R9: 构建/vet/测试全绿，既有能力无回归
  - **Type**: `rule`
  - **Covers**: AC-11
  - **Evidence**: R1 pass：build/vet/test/-race/windows 交叉编译全绿，gofmt 无差异

- [x] CP-U1: 与四类备份链路的集成一致性与复用度
  - **Type**: `rubric`
  - **Covers**: AC-13
  - **Scale**: 1-5
  - **Anchors**: 1 = 另起下载/身份逻辑、大量复制；3 = 主链路复用但有旁路；5 = 全部复用现有取源/重试/记录体系，四类对称接入
  - **Pass Threshold**: >= 4
  - **Evidence**: R1 4/5（四类对称、零平行下载实现、修复并入 TaskRecord；F1 属边界缺陷，I-1 后复评）

- [x] CP-U2: 终端四段报告与修复流程可用性
  - **Type**: `rubric`
  - **Covers**: AC-14
  - **Scale**: 1-5
  - **Anchors**: 1 = 原始日志堆砌；3 = 分段但缺汇总/风险提示；5 = 四段分明+汇总+风险提示+风格统一
  - **Pass Threshold**: >= 4
  - **Evidence**: R1 4/5（四段+表格+风险提示；变更段未展示摘要差异，不阻塞）

- [x] CP-U3: GB 级文件流式处理、内存克制、有进度
  - **Type**: `rubric`
  - **Covers**: AC-15
  - **Scale**: 1-5
  - **Anchors**: 1 = 整文件入内存/无进度；3 = 流式但无进度或 I/O 放大；5 = 流式+固定缓冲+进度
  - **Pass Threshold**: >= 4
  - **Evidence**: R1 4/5；实现方实测 1GiB 643ms/≈1592MiB/s/Alloc≈3.2MiB，256KiB 固定缓冲，CLI 有 mpb 进度

## Review History

### Review R2
- **Result**: `pass`
- **Evidence**:
  - 全新上下文第二轮审查，独立实跑 build/vet/test/-race/GOOS=windows/gofmt 全绿；在 /tmp 副本自设三相册交错场景复核 F1 已修复（Save 排序后 rebuildIndex，全仓库 Entries 原地变更点均维护下标）。
  - F2 修复确认：marker 识别在 inManagedScope 之后，mood/board 的 avatars/assets 标记零噪声（有/无账本两路径实测），media 内标记与相册域根 marker 不回归。
  - F3 修复确认：新计划落 repair.json 前独立 Confirm（默认否，拒绝即返回不落盘不登录）；继续既有计划沿用既有询问；plan.Save 严格先于 ensureLogin。
  - I-4 复核：ledgerSourceResolved 误判路径不可达（quarantine 已移走坏文件、条目只能在下载成功后更新、裁决前重新 LoadLedger）；IsVideo 在 RetryFailed 中确实影响死链重试/getinfo/HLS 兜底分支；HLS ctx 取消测试实跑通过。
  - 检查点：CP-R1~R9 全部 pass；CP-U1/U2/U3 均 4/5（>=4）。
  - 不阻塞 advisory：A1 Upsert 同来源改 rel 时旧 byRelPath 键残留（无消费者可观测，建议一行 delete）；A2 HLS 最终 rename 在 Windows 目标已存在时的理论边界（修复流程已先隔离，触发面窄）；A3 修复中取消时隔离区已有移动，建议文案明示（已采纳）；A4 空域根会落 0 条目账本（无害）。
- **Blocked By**: 无
- **Resume When**: 无需 R3。
- **R2 后加固（实现方采纳 advisory，已补测试与全量回归）**：
  - A1：Upsert 更新分支在 rel 变更时 delete 旧 byRelPath 键；新增 TestLedgerUpsertSourceRenameDropsOldRelIndex。
  - A3：CLI 修复中断日志明示隔离文件保留位置与续作行为。
  - 加固后 go build/vet/test -count=1 全绿。A2/A4 记录为后续可选项（不影响验收）。

### Review R1
- **Result**: `fail`
- **Evidence**:
  - 审查者在全新上下文通读全部工件并实跑 build/vet/test/-race/GOOS=windows；F1、F2 均以一次性程序在 /tmp 副本实跑取证（仓库零改动）。
  - **F1（high, actionable）**：`Ledger.Save()` 原地排序 Entries 后未重建 bySource/byRelPath，多相册增量备份在首个相册提交后，后续 SkipDecision/Upsert 命中错位下标——缺失文件可被判 healthy 离线跳过，Upsert 接管逻辑可误删无关健康条目。位置 ledger.go Save；复现：A/B 两相册→Save→删 B0→A2 提交→B0 SkipDecision 返回 healthy。
  - **F2（low, actionable）**：scanManagedFiles 的 marker 判定在 inManagedScope 之前，说说/留言板域根 avatars/*.resume.json 被误报未完成并可污染 repair.json。
  - **F3（low, actionable）**：CLI 选择修复后无独立 Confirm 直接落计划+登录，与 FR-9/TR-8.3 不符。
  - Advisory（不阻塞）：A1 bak 写失败被静默吞；A2 固定 tmp 名/无父目录 fsync（单进程假设可接受）；A3 修复裁决仅按 rel，扩展名变更会使计划项永久残留；A4 提交失败提示文案与实际补登记条件不符；A5 测试盲区（跨相册 Save、ctx 取消 HLS、范围外 marker）；A6 moodFailedItems 缺 IsVideo，视频 getinfo 兜底可能不触发。
  - 检查点：CP-R1/R4/R5/R8 fail；CP-R2/R3/R6/R7/R9 pass；rubric 4/4/4。
- **Blocked By**: 无
- **Resume When**: I-1~I-3 修复并回归后，启动全新审查 R2（I-4 顺带处理 A3/A4/A6 与 A5 测试盲区）
