// 完整性核验与修复的终端交互入口。核验全程不登录、不联网；只有用户显式选择修复时才请求登录。

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/qinjintian/qq-zone/internal/app"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	integrityVerifyAll  = "🧾 核验全部备份"
	integrityGoBack     = "↩ 返回上一级"
	integrityActRepair  = "repair"
	integrityActAdopt   = "adopt"
	integrityActDiscard = "discard"
)

// handleIntegrity 是免登录的完整性核验与修复入口。
func (c *CLI) handleIntegrity(ctx context.Context) {
	domains := app.DiscoverLedgerRoots()
	if len(domains) == 0 {
		c.logger.Info("📂 本机 storage/ 下还没有发现任何备份目录（相册/群相册/说说/留言板），先完成一次备份再来核验。")
		return
	}

	options := []string{integrityVerifyAll}
	domainMap := map[string]app.LedgerDomain{}
	for _, d := range domains {
		label := integrityDomainLabel(d)
		options = append(options, label)
		domainMap[label] = d
	}
	options = append(options, integrityGoBack)

	var selected string
	if err := askOne(&survey.Select{
		Message:  "选择要核验的备份（核验不登录、不联网）:",
		Options:  options,
		PageSize: 14,
	}, &selected, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "🧪"
		icons.SelectFocus.Text = "▶"
	})); err != nil || selected == integrityGoBack {
		return
	}

	if selected == integrityVerifyAll {
		for _, d := range domains {
			c.verifyOneDomain(ctx, d, false)
		}
		return
	}
	if d, ok := domainMap[selected]; ok {
		c.verifyOneDomain(ctx, d, true)
	}
}

// integrityDomainLabel 渲染单个域根的菜单行。
func integrityDomainLabel(d app.LedgerDomain) string {
	typeName := map[app.LedgerKind]string{
		app.LedgerKindAlbum:      "个人相册",
		app.LedgerKindGroupAlbum: "群相册",
		app.LedgerKindShuoshuo:   "说说",
		app.LedgerKindBoard:      "留言板",
	}[d.Kind]
	target := d.OwnerUin
	if d.Kind == app.LedgerKindGroupAlbum {
		target = d.OwnerUin + " / 群 " + d.GroupID
	}
	switch {
	case d.Corrupt:
		return fmt.Sprintf("[%s] %s | %s", typeName, target, color.RedString("账本损坏"))
	case d.HasLedger:
		return fmt.Sprintf("[%s] %s | 账本: %s（%d 条）", typeName, target, color.GreenString("已有"), d.EntryCount)
	default:
		return fmt.Sprintf("[%s] %s | 账本: %s", typeName, target, color.YellowString("无（可建基线）"))
	}
}

// verifyOneDomain 跑单个域根的核验并提供后续动作；interactive=false（全部核验）时只报告不进入修复。
func (c *CLI) verifyOneDomain(ctx context.Context, d app.LedgerDomain, interactive bool) {
	color.Cyan("\n━━━━━━━━━━━━━━━━━━ 核验：%s ━━━━━━━━━━━━━━━━━━", integrityDomainLabel(d))

	if d.Corrupt {
		c.logger.Errorf("❌ %s 的账本 ledger.json 与 ledger.bak 均无法解析。为避免覆盖可信记录，已跳过；请人工检查 .integrity/ 目录。", d.Root)
		return
	}

	if !d.HasLedger {
		c.offerBaselineOrScan(ctx, d)
		return
	}

	// 存在未完成修复计划时，先提示可直接续作。
	if app.HasPendingRepair(d.Root) {
		c.logger.Warn("📌 该备份存在上次未完成的修复计划（无凭证、断网或取消会保留，健康文件不会被重下）。")
		if interactive && c.askYesNo("是否直接继续执行修复计划？将移动损坏文件到隔离区并需要登录，健康文件不会重写。", true) {
			c.runRepair(ctx, d, nil, true)
			return
		}
	}

	report := c.runVerifyWithProgress(d)
	c.renderVerifyReport(d, report)

	if !interactive || report.ProblemCount() == 0 {
		return
	}

	c.offerPostVerifyActions(ctx, d, report)
}

// offerBaselineOrScan 处理无账本旧备份：显式建基线，或仅扫描现存媒体。
func (c *CLI) offerBaselineOrScan(ctx context.Context, d app.LedgerDomain) {
	var choice string
	if err := askOne(&survey.Select{
		Message: "该备份还没有完整性账本：",
		Options: []string{
			"🏗 建立本地基线（仅记录当前字节，来源未验证）",
			"🔍 仅扫描现存媒体与未完成项（不写账本）",
			integrityGoBack,
		},
	}, &choice); err != nil || choice == integrityGoBack {
		return
	}

	if strings.HasPrefix(choice, "🔍") {
		report := app.ScanRootWithoutLedger(d.Kind, d.Root, nil)
		c.renderVerifyReport(d, report)
		c.logger.Info(color.HiBlackString("提示：未登记文件可通过「建立本地基线」获得来源未验证登记；建基线不代表已与空间原件核对。"))
		return
	}

	c.logger.Warn("⚠️ " + app.BaselineNotice)
	if !c.askYesNo("确认只为当前磁盘字节建立「来源未验证」基线？", false) {
		return
	}
	l, res, err := app.EstablishBaseline(d.Kind, d.OwnerUin, d.GroupID, d.Root, nil)
	if err != nil {
		c.logger.Errorf("❌ 建立基线失败: %v", err)
		return
	}
	c.logger.Infof("✅ 基线已建立：%s（登记 %d 个媒体；账本 %s）", d.Root, res.EntryCount, app.LedgerPath(d.Root))
	if len(res.IncompleteList) > 0 {
		c.logger.Warnf("🚧 以下未完成文件未纳入基线（%d 个）: %s", len(res.IncompleteList), strings.Join(limitForDisplay(res.IncompleteList, 10), ", "))
	}
	_ = l
	// 建完基线立即核验一次，让用户看到健康（未验证）状态。
	report, err := app.VerifyRoot(d.Root, nil)
	if err == nil {
		c.renderVerifyReport(d, report)
		if report.ProblemCount() > 0 {
			c.offerPostVerifyActions(ctx, d, report)
		}
	}
}

// runVerifyWithProgress 用进度条执行核验；核验只读。
func (c *CLI) runVerifyWithProgress(d app.LedgerDomain) *app.VerifyReport {
	p := mpb.NewWithContext(context.Background())
	bar := p.AddBar(1,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name("本地核验 ", decor.WC{W: 16, C: decor.DindentRight}),
			decor.CountersNoUnit("%d / %d 个文件"),
		),
		mpb.AppendDecorators(decor.Percentage()),
	)
	var report *app.VerifyReport
	var verr error
	progress := func(done, total int, _ string) {
		bar.SetTotal(int64(total), false)
		bar.SetCurrent(int64(done))
	}
	report, verr = app.VerifyRoot(d.Root, progress)
	bar.SetTotal(-1, true)
	p.Wait()
	if verr != nil {
		c.logger.Errorf("❌ 核验失败: %v", verr)
		return nil
	}
	return report
}

// offerPostVerifyActions 报告之后的动作菜单：修复 / 纳入未登记 / 丢弃既有计划 / 返回。
func (c *CLI) offerPostVerifyActions(ctx context.Context, d app.LedgerDomain, report *app.VerifyReport) {
	var options []string
	options = append(options, "🔧 修复以上问题（只重建确认损坏/缺失项，需要登录）")
	if len(report.Section(app.VerifyUnregistered)) > 0 {
		options = append(options, "📥 把未登记项纳入基线（来源未验证，不修改文件）")
	}
	if app.HasPendingRepair(d.Root) {
		options = append(options, "🗑 丢弃上次未完成的修复计划")
	}
	options = append(options, integrityGoBack)

	var choice string
	if err := askOne(&survey.Select{Message: "接下来：", Options: options}, &choice); err != nil || choice == integrityGoBack {
		return
	}
	switch {
	case strings.HasPrefix(choice, "🔧"):
		plan := app.NewRepairPlan(d, report)
		if len(plan.Items) == 0 {
			c.logger.Info("没有可自动修复的项目（未登记项请用「纳入基线」）。")
			return
		}
		c.runRepair(ctx, d, plan, false)
	case strings.HasPrefix(choice, "📥"):
		c.adoptUnregistered(d, report)
	case strings.HasPrefix(choice, "🗑"):
		c.discardPlan(d)
	}
}

// runRepair 执行修复。alreadyConfirmed=true 表示用户已在「继续既有计划」处确认；
// 新计划必须在落盘前再做一次独立确认（FR-9）。
func (c *CLI) runRepair(ctx context.Context, d app.LedgerDomain, plan *app.RepairPlan, alreadyConfirmed bool) {
	if plan == nil {
		loaded, err := app.LoadRepairPlan(d.Root)
		if err != nil {
			c.logger.Errorf("❌ 没有可继续的修复计划: %v", err)
			return
		}
		plan = loaded
	}
	if len(plan.Items) == 0 {
		c.logger.Info("修复计划为空。")
		return
	}

	if !alreadyConfirmed {
		c.logger.Warn("修复将仅处理计划内的缺失/损坏/未完成项：损坏文件先移动到 .integrity/quarantine/ 隔离，" +
			"再通过该备份类型原有的下载与换源链路重建；健康文件不会被重写。修复需要登录。")
		if !c.askYesNo(fmt.Sprintf("确认开始修复这 %d 项？", len(plan.Items)), false) {
			c.logger.Info("已取消，未对文件做任何改动。")
			return
		}
	}

	// 先持久化计划，再登录：无凭证/取消/断网后下次都能继续，媒体文件未做任何改动。
	if err := plan.Save(); err != nil {
		c.logger.Errorf("❌ 修复计划无法落盘，已中止: %v", err)
		return
	}
	c.logger.Infof("🧾 修复计划已保存（%d 项）：%s；损坏/未完成文件将先隔离到 .integrity/quarantine/ 再重建，健康文件不会被重写。",
		len(plan.Items), app.RepairPlanPath(d.Root))

	if c.client == nil {
		if err := c.ensureLogin(ctx); err != nil {
			c.logger.Warn("⚠️ 未登录，修复计划已保留，下次可在同一入口继续。")
			return
		}
	}

	taskLogger := c.createTaskLogger(d.OwnerUin)
	record := app.NewTaskRecord(app.TaskModeRepair, c.client.QQ, d.OwnerUin, nil, c.config, true)
	if d.Kind == app.LedgerKindGroupAlbum {
		record.GroupID = d.GroupID
	}
	c.saveTaskRecord(record, nil, nil, app.TaskStatusPending)

	runner := app.NewOnlineRepairRunner(c.client, c.config, taskLogger)
	fmt.Println(color.HiBlackString("\n━━━━━━━━━━━━━━━━━━━━━━ 正在定点修复 ━━━━━━━━━━━━━━━━━━━━━━"))
	outcome, runErr := app.ExecuteRepair(ctx, d, plan, runner, taskLogger)
	fmt.Println(color.HiBlackString("━━━━━━━━━━━━━━━━━━━━━━ 修复结束 ━━━━━━━━━━━━━━━━━━━━━━"))

	status := c.determineTaskStatus(ctx, outcome.Result, runErr)
	if runErr != nil && status != app.TaskStatusCancelled {
		status = app.TaskStatusPartial
	}
	c.saveTaskRecord(record, outcome.Result, runErr, status)
	if runErr != nil {
		taskLogger.Errorf("❌ 修复中断（计划已保留，可继续；已隔离文件保留在 .integrity/quarantine/，续作会重新下载）: %v", runErr)
	}
	c.renderRepairOutcome(d, outcome)
}

// adoptUnregistered 把未登记媒体纳入既有账本（local、未验证）。
func (c *CLI) adoptUnregistered(d app.LedgerDomain, report *app.VerifyReport) {
	items := report.Section(app.VerifyUnregistered)
	c.logger.Warn("⚠️ " + app.BaselineNotice)
	if !c.askYesNo(fmt.Sprintf("确认把 %d 个未登记文件纳入基线（来源未验证、不修改文件）？", len(items)), false) {
		return
	}
	added, err := app.AdoptUnregistered(d.Root, report, nil)
	if err != nil {
		c.logger.Errorf("❌ 纳入基线失败: %v", err)
		return
	}
	c.logger.Infof("✅ 已纳入 %d 个未登记文件（来源未验证）。", added)
}

// discardPlan 经确认后丢弃既有修复计划。
func (c *CLI) discardPlan(d app.LedgerDomain) {
	plan, err := app.LoadRepairPlan(d.Root)
	if err != nil {
		return
	}
	if !c.askYesNo(fmt.Sprintf("确认丢弃 %d 项未完成修复计划？（不会删除任何媒体文件）", len(plan.Items)), false) {
		return
	}
	if err := plan.Remove(); err != nil {
		c.logger.Errorf("❌ 丢弃计划失败: %v", err)
		return
	}
	c.logger.Info("🧹 修复计划已丢弃。")
}

// renderRepairOutcome 打印修复结果汇总。
func (c *CLI) renderRepairOutcome(d app.LedgerDomain, o *app.RepairOutcome) {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	c.logger.Infof("🔧 修复结果：%s %d 项；%s %d 项；%s %d 项",
		green("已重建"), len(o.Resolved), red("仍失败"), len(o.Remaining), yellow("不可自动修复"), len(o.Unrepairable))
	if len(o.Unrepairable) > 0 {
		c.logger.Warn("以下项目缺少在线取源线索（多为基线身份），请对对应相册/说说重新执行一次在线备份：")
		for _, it := range o.Unrepairable {
			c.logger.Warnf("  - %s", it.RelPath)
		}
	}
	if len(o.Remaining) > 0 {
		c.logger.Info("计划已保留，下次进入本入口可继续修复；失败明细：")
		for _, it := range o.Remaining {
			c.logger.Infof("  - %s [%s] %s", it.RelPath, it.Reason, it.LastError)
		}
	}
}

// renderVerifyReport 在终端分段打印核验结果（缺失/变更/未登记/未完成 + 健康汇总）。
func (c *CLI) renderVerifyReport(d app.LedgerDomain, r *app.VerifyReport) {
	if r == nil {
		return
	}
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()

	header := fmt.Sprintf("\n%s %s（%s）", bold(cyan("🧪 本地核验报告")), integrityDomainLabel(d), d.Root)
	c.logger.Info(header)
	c.logger.Infof(" ✅ 健康：%s（来源已验证） + %s（基线/来源未验证）",
		green("%d", r.HealthyVerified), yellow("%d", r.HealthyUnverified))

	c.renderVerifySection(red("❌ 缺失"), r.Section(app.VerifyMissing), []string{"相对路径", "来源", "期望大小"})
	c.renderVerifySection(red("🔀 内容变更"), r.Section(app.VerifyChanged), []string{"相对路径", "期望 → 实际", "来源"})
	c.renderVerifySection(magenta("🚧 未完成"), r.Section(app.VerifyIncomplete), []string{"相对路径", "标记/说明"})
	c.renderVerifySection(yellow("❓ 未登记"), r.Section(app.VerifyUnregistered), []string{"相对路径", "大小"})

	c.logger.Infof("%s 问题合计 %d 项。核验未访问网络，也未修改任何文件或账本。",
		bold("━━ 汇总 ━━"), r.ProblemCount())
}

// renderVerifySection 打印一个分类的表格；空分类给一行绿色提示。
func (c *CLI) renderVerifySection(title string, items []app.VerifyItem, headers []string) {
	if len(items) == 0 {
		c.logger.Infof(" %s：无", title)
		return
	}
	c.logger.Infof(" %s（%d）:", title, len(items))
	tb := tablewriter.NewWriter(os.Stdout)
	tb.SetHeader(headers)
	tb.SetAutoWrapText(true)
	tb.SetColWidth(46)
	tb.SetBorder(false)
	tb.SetCenterSeparator("")
	tb.SetColumnSeparator("")
	tb.SetRowSeparator("")
	tb.SetHeaderLine(false)
	tb.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	tb.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, it := range items {
		switch it.Status {
		case app.VerifyMissing:
			tb.Append([]string{it.RelPath, it.SourceID, fmt.Sprintf("%d", it.WantSize)})
		case app.VerifyChanged:
			tb.Append([]string{it.RelPath, fmt.Sprintf("%d → %d", it.WantSize, it.GotSize), it.SourceID})
		case app.VerifyIncomplete:
			tb.Append([]string{it.RelPath, it.Detail})
		default:
			tb.Append([]string{it.RelPath, fmt.Sprintf("%d", it.GotSize)})
		}
	}
	tb.Render()
	fmt.Println()
}

// askYesNo 是一个简单的确认提问，默认值可设。
func (c *CLI) askYesNo(message string, defaultYes bool) bool {
	var ok bool
	if err := askOne(&survey.Confirm{Message: message, Default: defaultYes}, &ok); err != nil {
		return false
	}
	return ok
}

// limitForDisplay 截断过长列表用于终端提示。
func limitForDisplay(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(items[:n], fmt.Sprintf("…（其余 %d 项略）", len(items)-n))
}
