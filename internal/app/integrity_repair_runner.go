// 定点修复的在线执行器：完全复用四类备份现有的取源、多候选换源、断点续传与失败重试链路，
// 不另起任何下载实现。相册走白名单 Spider（增量 + 账本跳过），说说/留言板走 RetryFailed。

package app

import (
	"context"

	"github.com/qinjintian/qq-zone/internal/qzone"
	"go.uber.org/zap"
)

// OnlineRepairRunner 用已登录客户端执行修复重建。
type OnlineRepairRunner struct {
	client *qzone.Client
	config *Config
	logger *zap.SugaredLogger
}

// NewOnlineRepairRunner 构造在线修复执行器。
func NewOnlineRepairRunner(client *qzone.Client, config *Config, logger *zap.SugaredLogger) *OnlineRepairRunner {
	return &OnlineRepairRunner{client: client, config: config, logger: logger}
}

// RunAlbumRepair 以增量模式、相册白名单跑受影响相册：
// 缺失/已隔离的文件重新下载，健康文件由账本一致性闸门保证不重写。
func (r *OnlineRepairRunner) RunAlbumRepair(ctx context.Context, kind LedgerKind, ownerUin, groupID string, selectors []string) (*DownloadResult, error) {
	var spider *Spider
	if kind == LedgerKindGroupAlbum {
		spider = NewGroupSpider(r.client, r.config, selectors, groupID, "", r.logger)
	} else {
		spider = NewSpider(r.client, r.config, selectors, r.logger)
	}
	return spider.Download(ctx, ownerUin, true)
}

// RunMoodRepair 复用说说失败重试链路：按 tid 刷详情换地址、多候选 + getinfo 换源。
func (r *OnlineRepairRunner) RunMoodRepair(ctx context.Context, ownerUin string, items []FailedItem) (*DownloadResult, error) {
	return NewMoodBackup(r.client, r.config, r.logger).RetryFailed(ctx, ownerUin, items)
}

// RunBoardRepair 复用留言板失败重试链路。
func (r *OnlineRepairRunner) RunBoardRepair(ctx context.Context, ownerUin string, items []FailedItem) (*DownloadResult, error) {
	return NewBoardBackup(r.client, r.config, r.logger).RetryFailed(ctx, ownerUin, items)
}
