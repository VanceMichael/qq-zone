// 备份预检报告的终端展示与确认闸门。个人相册和群相册在写盘前共用这一套交互。

package cli

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/qinjintian/qq-zone/internal/app"
	"github.com/qinjintian/qq-zone/internal/pkg/util"
)

const (
	planProceed   = iota // 用户确认，按冻结清单开始备份
	planCancelled        // 用户在确认阶段取消：不留任何任务和文件副作用
	planRejected         // 已知待写字节超过目标卷可用空间：硬性拒绝
)

// reviewBackupPlan 展示预检容量账并要求用户明确确认。
// 空间不足时不给确认机会（planRejected）；存在未知项时会额外警示，但确认后仍可执行。
func (c *CLI) reviewBackupPlan(plan *app.BackupPlan) int {
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	gray := color.New(color.FgWhite, color.Faint).SprintFunc()

	bar := gray("────────────────────────────────────────────────────────────")
	var out string
	out += "\n" + bold(cyan("🧾 备份预检报告")) + "  " + gray("（确认前不会创建任务记录、目录或任何文件）") + "\n"
	out += bar + "\n"
	out += fmt.Sprintf(" 💽 %-12s %s\n", "目标卷:", cyan(plan.VolumePath))
	out += fmt.Sprintf(" 📈 %-12s %s  %s\n", "卷可用空间:", green(util.FormatBytes(plan.FreeBytes)), gray("(卷总容量 "+util.FormatBytes(plan.VolumeTotal)+")"))
	out += fmt.Sprintf(" 🧮 %-12s %s 项\n", "媒体总数:", bold(plan.Total))
	out += fmt.Sprintf(" 🆕 %-12s %s 项，完整下载需 %s\n", "新下载:", green(plan.NewCount), green(util.FormatBytes(plan.NewBytes)))
	out += fmt.Sprintf(" ▶️  %-12s %s 项，本地已有 %s，还需写入 %s\n",
		"可续传:", yellow(plan.ResumeCount),
		util.FormatBytes(plan.ResumeHaveBytes), yellow(util.FormatBytes(plan.ResumeBytes)))
	out += fmt.Sprintf(" ⏭️  %-12s %s 项\n", "完整跳过:", yellow(plan.SkipCount))
	out += fmt.Sprintf(" ❓ %-12s %s 项 %s\n", "大小未知:", yellow(plan.UnknownCount), gray("（HEAD 失败 / HLS / 无可靠长度，不计入下方字节）"))
	out += bar + "\n"
	out += fmt.Sprintf(" ⛽ %-12s %s\n", "已知至少写入:", bold(util.FormatBytes(plan.KnownRemaining)))

	if plan.SpaceShortage() {
		out += "\n" + red("❌ 空间不足，已拒绝执行：") +
			fmt.Sprintf("\n   已知至少要写入 %s，但目标卷只剩 %s。",
				red(util.FormatBytes(plan.KnownRemaining)), red(util.FormatBytes(plan.FreeBytes))) +
			"\n   已在任务记录、目录、临时文件和媒体内容产生之前中止；未知项还可能进一步增加占用。"
		c.logger.Info(out)
		return planRejected
	}

	if plan.UnknownCount > 0 {
		out += "\n" + yellow("⚠️  存在大小未知的媒体：") +
			fmt.Sprintf(" %s 项未计入已知字节，实际占用可能高于 %s。", yellow(plan.UnknownCount), util.FormatBytes(plan.KnownRemaining)) +
			"\n   请确认卷上预留了足够余量；确认后这些项仍会尝试下载。"
	}

	c.logger.Info(out)

	confirm := false
	if err := askOne(&survey.Confirm{
		Message: color.New(color.FgCyan, color.Bold).Sprint(
			"是否按这份冻结清单开始备份？（确认后只下载清单中的媒体，不会二次拉取加入新照片）"),
		Default: false,
	}, &confirm, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "❓"
		icons.SelectFocus.Text = "▶"
	})); err != nil || !confirm {
		return planCancelled
	}
	return planProceed
}
