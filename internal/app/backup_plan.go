// 备份预检（写盘前容量规划）：拉取完整分页清单，按与真正下载相同的路径、时间线和本地索引规则，
// 把每项媒体分为「新下载 / 完整跳过 / 可续传 / 大小未知」，并在用户确认前给出字节级容量账。
//
// 硬约束：整个预检只读，不创建任务记录、目录、临时文件，也不写媒体内容；
// 确认后的下载必须消费这里冻结的同一份清单（见 Spider.DownloadPlan）。

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	ihttp "github.com/qinjintian/qq-zone/internal/net/http"
	"github.com/qinjintian/qq-zone/internal/pkg/util"
	"github.com/qinjintian/qq-zone/internal/qzone"
	"github.com/tidwall/gjson"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/sync/errgroup"
)

// planProbeTimeout 单个媒体大小探测（HEAD + 首字节 GET）的超时。
const planProbeTimeout = 20 * time.Second

// PlanItemKind 是预检给单个媒体定的处置类别。
type PlanItemKind string

const (
	// PlanItemNew 本地没有完整文件，需要整份新下载，按远端完整长度计字节。
	PlanItemNew PlanItemKind = "new"
	// PlanItemSkip 增量模式下本地已有不小于远端长度的同名文件，会被完整跳过，计 0 字节。
	PlanItemSkip PlanItemKind = "skip"
	// PlanItemResume 本地有与同一 URL 绑定的半成品，可以 Range 续传，只计尚未写入的字节。
	PlanItemResume PlanItemKind = "resume"
	// PlanItemUnknown HEAD/首字节探测失败、仅 HLS/getinfo 源或没有可靠 Content-Length。
	// 明确单列、不计入已知字节，绝不能当成 0 字节。
	PlanItemUnknown PlanItemKind = "unknown"
)

// PlanItem 是单个媒体在预检时的容量账。
type PlanItem struct {
	AlbumName  string       // 所属相册名（展示用）
	Name       string       // 预测的本地文件名
	IsVideo    bool         // 是否视频
	Kind       PlanItemKind // 处置类别
	RemoteSize int64        // 探测到的远端完整字节数；未知为 0
	LocalSize  int64        // 本地已存在的字节数（跳过/续传判定依据）
	Remaining  int64        // 本次预计还要写入的字节数；跳过和未知为 0
	Reason     string       // 未知原因等说明
}

// FrozenAlbum 是确认时刻冻结的一个相册：相册元数据 + 完整分页媒体清单。
// DownloadPlan 只消费它，不再二次拉取，因此执行时不会静默混入确认后新增的照片。
type FrozenAlbum struct {
	Album  gjson.Result
	Photos []gjson.Result
}

// BackupPlan 是一份只读预检的结果，也是确认后下载执行的唯一清单来源。
type BackupPlan struct {
	TargetUin string // 被备份空间的 QQ（群相册时即当前登录 QQ）
	GroupID   string // 非空表示群相册
	GroupName string
	Exclude   bool // 当时选择的增量模式

	Albums []FrozenAlbum // 冻结的相册与媒体清单
	Items  []PlanItem    // 与 Albums 展开顺序一致的逐项容量账

	Total        int // 媒体总数
	NewCount     int // 新下载项数
	SkipCount    int // 完整跳过项数
	ResumeCount  int // 可续传项数
	UnknownCount int // 大小未知项数

	NewBytes        int64 // 新下载项的完整字节合计
	ResumeBytes     int64 // 续传项「尚未写入」字节合计
	ResumeHaveBytes int64 // 续传项本地已存在的字节（仅展示）
	KnownRemaining  int64 // 已知至少要写入的字节 = NewBytes + ResumeBytes

	VolumePath  string // 实际用于容量查询的现存目录
	FreeBytes   int64  // 目标卷普通用户可用字节
	VolumeTotal int64  // 目标卷总字节
	BuiltAt     time.Time
}

// SpaceShortage 判断已知待写字节是否已超过目标卷可用空间；为真时必须在产生任何副作用前拒绝。
func (p *BackupPlan) SpaceShortage() bool {
	return p != nil && p.KnownRemaining > p.FreeBytes
}

// planSlot 是并发探测队列里的一个工作槽：媒体身份 + 它所属相册的只读本地索引。
type planSlot struct {
	album      gjson.Result
	photo      gjson.Result
	albumPath  string
	localFiles map[string]string
}

// planCandidateProbe 单条候选地址的探测结果：
// headSize/hasHead 与 shouldSkipExisting 同源（只带 cookie 的 HEAD），专供「完整跳过」判定；
// size 是容量账使用的可靠完整长度（HEAD 优先，HEAD 不行再走与下载同源的 Range GET）。
// index 是该候选在候选列表中的优先级序号，续传只可能发生在最高优先级候选上。
type planCandidateProbe struct {
	index    int
	url      string
	headSize int64
	hasHead  bool
	size     int64
}

// BuildBackupPlan 在写盘前完成预检：查容量 → 拉完整分页清单 → 逐项分类与计字节。
// 整个过程不创建目录、临时文件或任务记录。分页中途失败返回错误，整份计划作废；
// 单个媒体探测失败只产生一个 unknown 项，不影响其他项。
func (s *Spider) BuildBackupPlan(ctx context.Context, targetUin string, selected []gjson.Result, exclude bool) (*BackupPlan, error) {
	plan := &BackupPlan{
		TargetUin: strings.TrimSpace(targetUin),
		GroupID:   s.groupID,
		GroupName: s.groupName,
		Exclude:   exclude,
		BuiltAt:   time.Now(),
	}

	// 容量先查：查不到就显式报错并中止，保证零写入。
	root := s.storageRoot(targetUin)
	mount, free, total, err := util.VolumeFreeSpace(root)
	if err != nil {
		return nil, fmt.Errorf("无法获取目标卷可用空间，预检中止（尚未写入任何文件）: %w", err)
	}
	plan.VolumePath, plan.FreeBytes, plan.VolumeTotal = mount, free, total

	// 第一步：把入选相册的媒体清单全部拉完。任一页失败都让整份计划无效，不能拿半截清单做容量账。
	p := mpb.NewWithContext(ctx)
	wait := waitSpinner(p, "预检: 正在拉取完整媒体清单")
	for _, album := range selected {
		select {
		case <-ctx.Done():
			stopWaitSpinner(wait)
			p.Wait()
			return nil, ctx.Err()
		default:
		}

		photos, fetchErr := s.fetchPlanPhotos(ctx, targetUin, album.Get("id").String())
		if fetchErr != nil {
			stopWaitSpinner(wait)
			p.Wait()
			return nil, fmt.Errorf("相册 [%s] 的分页清单拉取失败，整份预检计划无效，网络恢复后请重新规划: %w",
				album.Get("name").String(), fetchErr)
		}
		plan.Albums = append(plan.Albums, FrozenAlbum{Album: album, Photos: photos})
	}
	stopWaitSpinner(wait)

	// 第二步：展开工作槽。本地索引只读构建，绝不 MkdirAll / RemoveAll。
	slots := make([]planSlot, 0)
	for _, frozen := range plan.Albums {
		albumPath := s.plannedMediaDir(targetUin, frozen.Album.Get("name").String())
		var localFiles map[string]string
		if exclude {
			localFiles = readOnlyMediaIndex(albumPath)
		}
		for _, photo := range frozen.Photos {
			slots = append(slots, planSlot{
				album:      frozen.Album,
				photo:      photo,
				albumPath:  albumPath,
				localFiles: localFiles,
			})
		}
	}
	plan.Total = len(slots)
	plan.Items = make([]PlanItem, len(slots))

	if len(slots) > 0 {
		probeBar := p.AddBar(int64(len(slots)),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(
				decor.Name("预检: 探测媒体大小 ", decor.WC{W: 24, C: decor.DindentRight}),
				decor.CountersNoUnit("%d / %d"),
			),
			mpb.AppendDecorators(
				decor.Percentage(),
				decor.Name(" ] "),
				decor.OnComplete(decor.Name("", decor.WC{W: 5}), "Done!"),
			),
		)

		workers := int32(s.config.TaskLimit)
		if workers < 1 {
			workers = 10
		}

		g, gCtx := errgroup.WithContext(ctx)
		jobCh := make(chan int)
		g.Go(func() error {
			defer close(jobCh)
			for i := range slots {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				case jobCh <- i:
				}
			}
			return nil
		})
		for w := int32(0); w < workers; w++ {
			g.Go(func() error {
				for idx := range jobCh {
					slot := slots[idx]
					plan.Items[idx] = s.planOneItem(gCtx, targetUin, slot.album, slot.photo, slot.localFiles, exclude)
					probeBar.Increment()
				}
				return nil
			})
		}
		waitErr := g.Wait()
		probeBar.SetCurrent(int64(len(slots)))
		probeBar.SetTotal(int64(len(slots)), true)
		p.Wait()
		if waitErr != nil {
			return nil, waitErr
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	} else {
		p.Wait()
	}

	plan.aggregate()
	return plan, nil
}

// fetchPlanPhotos 拉单个相册的完整分页清单；测试可用 planListPhotos 替换整条网络链路。
func (s *Spider) fetchPlanPhotos(ctx context.Context, targetUin, albumID string) ([]gjson.Result, error) {
	if s.planListPhotos != nil {
		return s.planListPhotos(ctx, targetUin, s.groupID, albumID)
	}
	if s.isGroupMode() {
		return s.client.GetGroupPhotoList(ctx, s.groupID, albumID)
	}
	return s.client.GetPhotoList(ctx, targetUin, albumID)
}

// planOneItem 按与 downloadItem / shouldSkipExisting 相同的规则给单个媒体分类计字节。
func (s *Spider) planOneItem(ctx context.Context, targetUin string, album, photo gjson.Result, localFiles map[string]string, exclude bool) PlanItem {
	spec := s.buildMediaSpec(photo)
	filename := spec.imageName
	if spec.isVideo {
		filename = spec.videoName
	}
	item := PlanItem{
		AlbumName: album.Get("name").String(),
		Name:      filename,
		IsVideo:   spec.isVideo,
		Kind:      PlanItemUnknown,
	}

	candidates, _, resolveErr := s.resolveMediaCandidates(ctx, targetUin, album, spec, photo)
	if resolveErr != nil || len(candidates) == 0 {
		if resolveErr == nil {
			resolveErr = fmt.Errorf("无可用下载地址")
		}
		item.Reason = "媒体源解析失败: " + resolveErr.Error()
		return item
	}

	// 增量模式下查本地同名（去扩展名）文件；非增量模式目录会在执行时整体重下，不看本地。
	var (
		localSize  int64
		sidecarURI string
		hasSidecar bool
	)
	if exclude {
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
		if existingPath, ok := localFiles[base]; ok {
			localSize, sidecarURI, hasSidecar = ihttp.ResumeSidecar(existingPath)
		}
	}

	// 按候选优先级探测：getinfo / HLS 与 shouldSkipExisting 一样视为不可靠长度来源直接跳过。
	hits := make([]planCandidateProbe, 0, len(candidates))
	sawUnreliable := false
	for i, cand := range candidates {
		if cand.Kind == qzone.VideoSourceGetInfo || ihttp.IsHLSURL(cand.URL) {
			sawUnreliable = true
			continue
		}
		if hit, ok := s.probeCandidatePlan(ctx, targetUin, i, cand, spec.isVideo); ok {
			hits = append(hits, hit)
		}
	}

	if len(hits) == 0 {
		if sawUnreliable {
			item.Reason = "仅 HLS/getinfo 等无可靠长度的来源，大小未知（不计入已知字节）"
		} else {
			item.Reason = "HEAD/首字节探测失败或无可靠 Content-Length，大小未知（不计入已知字节）"
		}
		return item
	}

	first := hits[0]
	item.RemoteSize = first.size

	if exclude && localSize > 0 {
		// 与 shouldSkipExisting 完全一致：任一候选的「仅 cookie HEAD」声明长度不超过本地 → 完整跳过。
		for _, hit := range hits {
			if hit.hasHead && hit.headSize <= localSize {
				item.Kind = PlanItemSkip
				item.LocalSize = localSize
				item.Remaining = 0
				return item
			}
		}
		if first.size > localSize {
			// 只有最高优先级候选、且 sidecar 与该 URL 匹配的半成品能续 Range；
			// 否则下载层会先按换源规则删掉半成品重下（见 http.Download），按整份新下载计字节更安全。
			if first.index == 0 && hasSidecar && sidecarURI == first.url {
				item.Kind = PlanItemResume
				item.LocalSize = localSize
				item.Remaining = first.size - localSize
				return item
			}
			item.Kind = PlanItemNew
			item.Remaining = first.size
			return item
		}
	}

	item.Kind = PlanItemNew
	item.Remaining = first.size
	return item
}

// probeCandidatePlan 探测一条候选地址。
// hasHead/headSize 来自与 shouldSkipExisting 相同的「仅 cookie HEAD」；
// size 是容量账长度（HEAD 优先，失败再用真实下载头做 HEAD+Range GET）。
// 视频带 Cookie 被 403/404 时与 downloadCandidate 一样去掉 Cookie 再试一次。
func (s *Spider) probeCandidatePlan(ctx context.Context, targetUin string, index int, cand qzone.VideoCandidate, isVideo bool) (planCandidateProbe, bool) {
	hit := planCandidateProbe{index: index, url: cand.URL}

	headCtx, headCancel := context.WithTimeout(ctx, planProbeTimeout)
	headSize, headErr := s.client.Http.ProbeSizeHead(headCtx, cand.URL, map[string]string{"cookie": s.client.Cookie})
	headCancel()
	if headErr == nil && headSize > 0 {
		hit.hasHead = true
		hit.headSize = headSize
		hit.size = headSize
		return hit, true
	}

	call := func(h map[string]string) (int64, error) {
		pctx, cancel := context.WithTimeout(ctx, planProbeTimeout)
		defer cancel()
		return s.client.Http.ProbeSize(pctx, cand.URL, h)
	}
	downloadHeaders := s.buildDownloadHeaders(targetUin, cand.URL, isVideo, cand.Kind)
	size, err := call(downloadHeaders)
	if err == nil && size > 0 {
		hit.size = size
		return hit, true
	}
	if isVideo && err != nil && downloadHeaders["cookie"] != "" && ihttp.IsDeadURL(err) {
		anon := cloneHeaders(downloadHeaders)
		delete(anon, "cookie")
		if size2, err2 := call(anon); err2 == nil && size2 > 0 {
			hit.size = size2
			return hit, true
		}
	}
	return hit, false
}

// aggregate 汇总各类计数与字节账。
func (p *BackupPlan) aggregate() {
	p.Total = len(p.Items)
	for _, item := range p.Items {
		switch item.Kind {
		case PlanItemNew:
			p.NewCount++
			p.NewBytes += item.Remaining
		case PlanItemResume:
			p.ResumeCount++
			p.ResumeBytes += item.Remaining
			p.ResumeHaveBytes += item.LocalSize
		case PlanItemSkip:
			p.SkipCount++
		default:
			p.UnknownCount++
		}
	}
	p.KnownRemaining = p.NewBytes + p.ResumeBytes
}

// storageRoot 是该目标所有媒体写入位置的公共根目录，容量查询就挂在它所在的卷上。
func (s *Spider) storageRoot(targetUin string) string {
	if s.isGroupMode() {
		return filepath.Join("storage", "qzone", targetUin, "qun", sanitizePath(s.groupID))
	}
	return filepath.Join("storage", "qzone", targetUin, "album")
}

// plannedMediaDir 只推导相册目录而不创建它（与 buildMediaDir 的路径规则一致，但零写入）。
// 目录已存在时按真实目录取（含历史哈希兜底目录）；尚不存在时返回首选路径，留给确认后的下载去建。
func (s *Spider) plannedMediaDir(targetUin, albumName string) string {
	baseDir := filepath.Join("storage", "qzone", targetUin, "album")
	if s.isGroupMode() {
		baseDir = filepath.Join("storage", "qzone", targetUin, "qun", sanitizePath(s.groupID))
	}
	preferred := filepath.Join(baseDir, sanitizePath(albumName))
	if util.IsDir(preferred) {
		return preferred
	}
	hashed := filepath.Join(baseDir, util.MD5(albumName)[8:24])
	if util.IsDir(hashed) {
		return hashed
	}
	return preferred
}

// readOnlyMediaIndex 与 buildLocalFileIndex 的增量索引规则相同（去扩展名建键、递归时间线子目录），
// 但只读：不建目录、不清目录。
func readOnlyMediaIndex(albumPath string) map[string]string {
	localFiles := make(map[string]string)
	files, _ := util.ListFiles(albumPath)
	for _, f := range files {
		name := filepath.Base(f)
		if idx := strings.LastIndex(name, "."); idx != -1 {
			name = name[:idx]
		}
		localFiles[name] = f
	}
	return localFiles
}
