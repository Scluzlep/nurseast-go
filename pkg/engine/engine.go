package engine

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"nurseast-go/pkg/api"
	"nurseast-go/pkg/config"
)

type EngineState string

const (
	StateIdle      EngineState = "idle"
	StateRunning   EngineState = "running"
	StatePaused    EngineState = "paused"
	StateStopping  EngineState = "stopping"
	StateStopped   EngineState = "stopped"
	StateCompleted EngineState = "completed"
	StateFailed    EngineState = "failed"
)

type Engine struct {
	state      EngineState
	cancelFunc context.CancelFunc
	pauseCond  *sync.Cond
	isPaused   bool
	mu         sync.RWMutex
}

func newEngine() *Engine {
	e := &Engine{state: StateIdle}
	e.pauseCond = sync.NewCond(&e.mu)
	return e
}

var GlobalEngine = newEngine()

func (e *Engine) GetState() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Start 启动批量刷课任务
func (e *Engine) Start(planIDs []int64, client *api.Client, cfg *config.Config) error {
	if len(planIDs) == 0 || client == nil || cfg == nil {
		return fmt.Errorf("缺少课程或任务配置")
	}
	e.mu.Lock()
	if e.state == StateRunning || e.state == StatePaused || e.state == StateStopping {
		e.mu.Unlock()
		return fmt.Errorf("当前任务仍在运行或正在结束")
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancelFunc = cancel
	e.isPaused = false
	e.state = StateRunning
	Hub.Publish(Event{
		Type: EventStatusChange,
		Data: StatusPayload{Status: string(StateRunning)},
	})
	e.mu.Unlock()

	planIDs = append([]int64(nil), planIDs...)
	configSnapshot := *cfg
	taskClient := api.NewClient(client.GetAccessToken())
	go e.run(ctx, planIDs, taskClient, &configSnapshot)
	return nil
}

// Pause 暂停任务
func (e *Engine) Pause() {
	e.mu.Lock()
	if e.state != StateRunning {
		e.mu.Unlock()
		return
	}
	e.isPaused = true
	e.state = StatePaused
	Hub.Publish(Event{
		Type: EventStatusChange,
		Data: StatusPayload{Status: string(StatePaused)},
	})
	e.mu.Unlock()
	EmitLog("warn", "⏸️ 任务已暂停")
}

// Resume 继续任务
func (e *Engine) Resume() {
	e.mu.Lock()
	if e.state != StatePaused {
		e.mu.Unlock()
		return
	}
	e.isPaused = false
	e.state = StateRunning
	e.pauseCond.Broadcast()
	Hub.Publish(Event{
		Type: EventStatusChange,
		Data: StatusPayload{Status: string(StateRunning)},
	})
	e.mu.Unlock()
	EmitLog("info", "▶️ 任务已恢复继续运行")
}

// Stop 停止任务
func (e *Engine) Stop() {
	e.mu.Lock()
	if e.state != StateRunning && e.state != StatePaused {
		e.mu.Unlock()
		return
	}
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	e.isPaused = false
	e.state = StateStopping
	e.pauseCond.Broadcast()
	Hub.Publish(Event{
		Type: EventStatusChange,
		Data: StatusPayload{Status: string(StateStopping)},
	})
	e.mu.Unlock()
	EmitLog("warn", "⏹️ 任务已被用户手动终止")
}

func (e *Engine) checkPause(ctx context.Context) bool {
	e.mu.Lock()
	for e.isPaused && ctx.Err() == nil {
		e.pauseCond.Wait()
	}
	active := ctx.Err() == nil
	e.mu.Unlock()
	return active
}

func (e *Engine) publishProgress(ctx context.Context, event Event) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil || e.state == StateStopping {
		return false
	}
	Hub.Publish(event)
	return true
}

func (e *Engine) finishRun(ctx context.Context, total, succeeded, failed int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cancelFunc = nil
	if ctx.Err() != nil {
		e.state = StateStopped
	} else if failed > 0 {
		e.state = StateFailed
		Hub.Publish(Event{Type: EventOverallProgress, Data: OverallProgressPayload{
			Current: succeeded, Total: total, Percentage: succeeded * 100 / total,
			CurrentTitle: fmt.Sprintf("部分完成：%d 成功，%d 失败", succeeded, failed),
		}})
		EmitLog("error", fmt.Sprintf("任务结束：%d 个课程完成，%d 个课程未完成", succeeded, failed))
	} else {
		e.state = StateCompleted
		Hub.Publish(Event{Type: EventOverallProgress, Data: OverallProgressPayload{
			Current: total, Total: total, Percentage: 100, CurrentTitle: "全部完成",
		}})
		EmitLog("success", "所有已选课程任务已全部刷完！")
	}
	Hub.Publish(Event{Type: EventStatusChange, Data: StatusPayload{Status: string(e.state)}})
}

func parseTimeToSeconds(timeStr string) int64 {
	if timeStr == "" {
		return 0
	}
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		m, _ := strconv.ParseInt(parts[0], 10, 64)
		s, _ := strconv.ParseInt(parts[1], 10, 64)
		return m*60 + s
	} else if len(parts) == 3 {
		h, _ := strconv.ParseInt(parts[0], 10, 64)
		m, _ := strconv.ParseInt(parts[1], 10, 64)
		s, _ := strconv.ParseInt(parts[2], 10, 64)
		return h*3600 + m*60 + s
	}
	return 0
}

func confirmVideoComplete(ctx context.Context, client *api.Client, planID, vid int64) (bool, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		detail, err := client.GetVideoDetailContext(ctx, planID)
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		for _, video := range detail.VideoDetails {
			if video.VID == vid && video.VideoDurationSecond > 0 &&
				parseTimeToSeconds(video.UseTime) >= video.VideoDurationSecond {
				return true, nil
			}
		}
	}
	return false, lastErr
}

func (e *Engine) run(ctx context.Context, planIDs []int64, client *api.Client, cfg *config.Config) {
	total := len(planIDs)
	succeeded, failed := 0, 0
	defer func() { e.finishRun(ctx, total, succeeded, failed) }()
	EmitLog("info", fmt.Sprintf("自动刷课任务开始，共待处理 %d 个课程项目", total))

	for idx, planID := range planIDs {
		if !e.checkPause(ctx) {
			return
		}

		currentNum := idx + 1
		percent := int((float64(idx) / float64(total)) * 100)
		if !e.publishProgress(ctx, Event{
			Type: EventOverallProgress,
			Data: OverallProgressPayload{
				Current:      currentNum,
				Total:        total,
				Percentage:   percent,
				CurrentTitle: fmt.Sprintf("项目 ID: %d", planID),
			},
		}) {
			return
		}

		EmitLog("info", fmt.Sprintf("[%d/%d] 正在检索项目 ID: %d 详情...", currentNum, total, planID))

		detailData, err := client.GetVideoDetailContext(ctx, planID)
		if !e.checkPause(ctx) {
			return
		}
		if err != nil {
			EmitLog("error", fmt.Sprintf("[%d/%d] 获取项目 %d 详情失败: %v", currentNum, total, planID, err))
			failed++
			continue
		}

		if len(detailData.VideoDetails) == 0 {
			EmitLog("warn", fmt.Sprintf("[%d/%d] 项目 %d 无视频，无法确认已完成", currentNum, total, planID))
			failed++
			continue
		}

		// 依次处理该项目下的所有视频
		planFailed := false
		for _, video := range detailData.VideoDetails {
			if !e.checkPause(ctx) {
				return
			}

			totalDuration := video.VideoDurationSecond
			currentProgress := float64(video.LastTimePoint)
			usedSeconds := parseTimeToSeconds(video.UseTime)

			videoTitle := video.Title
			if videoTitle == "" {
				videoTitle = fmt.Sprintf("VID-%d", video.VID)
			}

			if !e.publishProgress(ctx, Event{
				Type: EventOverallProgress,
				Data: OverallProgressPayload{
					Current:      currentNum,
					Total:        total,
					Percentage:   percent,
					CurrentTitle: videoTitle,
				},
			}) {
				return
			}

			EmitLog("info", fmt.Sprintf("检查视频: 《%s》 (总长 %ds, 已观看 %ds, 历史点 %.1fs)",
				videoTitle, totalDuration, usedSeconds, currentProgress))
			if totalDuration <= 0 {
				EmitLog("error", fmt.Sprintf("视频《%s》时长无效，无法确认完成", videoTitle))
				planFailed = true
				break
			}

			if usedSeconds >= totalDuration {
				EmitLog("success", fmt.Sprintf("视频《%s》已完全达标，自动跳过", videoTitle))
				if !e.publishProgress(ctx, Event{
					Type: EventVideoProgress,
					Data: VideoProgressPayload{
						VID:            video.VID,
						Title:          videoTitle,
						CurrentSeconds: totalDuration,
						TotalSeconds:   totalDuration,
						Percentage:     100,
					},
				}) {
					return
				}
				continue
			}
			videoUserID, ok := detailData.VideoUserID[strconv.FormatInt(video.VID, 10)]
			if !ok {
				EmitLog("error", fmt.Sprintf("视频《%s》缺少对应的播放记录 ID", videoTitle))
				planFailed = true
				break
			}
			if currentProgress >= float64(totalDuration) {
				EmitLog("error", fmt.Sprintf("视频《%s》播放点已到末尾，但观看时长未达标", videoTitle))
				planFailed = true
				break
			}
			if currentProgress < 0 {
				currentProgress = 0
			}

			// 分段模拟观看
			videoComplete := false
			for currentProgress < float64(totalDuration) {
				if !e.checkPause(ctx) {
					return
				}

				// 随机分片时长
				delta := cfg.MaxChunkSec - cfg.MinChunkSec
				if delta <= 0 {
					delta = 0.6
				}
				chunkDuration := cfg.MinChunkSec + rand.Float64()*delta

				startChunk := int64(currentProgress)
				endChunkFloat := currentProgress + chunkDuration
				if endChunkFloat > float64(totalDuration) {
					endChunkFloat = float64(totalDuration)
				}
				endChunkInt := int64(endChunkFloat)

				if endChunkInt <= startChunk {
					EmitLog("error", fmt.Sprintf("视频《%s》进度无法继续推进", videoTitle))
					planFailed = true
					break
				}

				videoPercent := 0
				if totalDuration > 0 {
					videoPercent = int((float64(endChunkInt) / float64(totalDuration)) * 100)
				}
				if videoPercent >= 100 {
					videoPercent = 99
				}

				EmitLog("info", fmt.Sprintf("上报视频分片: %ds -> %ds (切片 %.1fs)...",
					startChunk, endChunkInt, chunkDuration))

				ok, msg, err := client.FlushVideoContext(ctx, planID, video.VID, videoUserID, startChunk, endChunkInt)
				if !e.checkPause(ctx) {
					return
				}
				if err != nil {
					EmitLog("error", fmt.Sprintf("上报进度异常: %v", err))
					planFailed = true
					break
				}
				if !ok {
					EmitLog("error", fmt.Sprintf("服务器拒绝进度上报: %s", msg))
					planFailed = true
					break
				}

				currentProgress = endChunkFloat
				if !e.publishProgress(ctx, Event{
					Type: EventVideoProgress,
					Data: VideoProgressPayload{
						VID:            video.VID,
						Title:          videoTitle,
						CurrentSeconds: endChunkInt,
						TotalSeconds:   totalDuration,
						Percentage:     videoPercent,
					},
				}) {
					return
				}
				if endChunkInt >= totalDuration {
					confirmed, confirmErr := confirmVideoComplete(ctx, client, planID, video.VID)
					if !e.checkPause(ctx) {
						return
					}
					if !confirmed {
						if confirmErr != nil {
							EmitLog("error", fmt.Sprintf("核对视频《%s》观看进度失败: %v", videoTitle, confirmErr))
						} else {
							EmitLog("error", fmt.Sprintf("服务器尚未确认视频《%s》观看完成", videoTitle))
						}
						planFailed = true
						break
					}
					videoComplete = true
					break
				}

				// 模拟真实观看时间休眠
				sleepDuration := time.Duration(chunkDuration * float64(time.Second))
				select {
				case <-ctx.Done():
					return
				case <-time.After(sleepDuration):
				}
			}

			if !videoComplete {
				if !planFailed {
					EmitLog("error", fmt.Sprintf("视频《%s》未能完成", videoTitle))
				}
				planFailed = true
				break
			}
			if !e.publishProgress(ctx, Event{
				Type: EventVideoProgress,
				Data: VideoProgressPayload{
					VID:            video.VID,
					Title:          videoTitle,
					CurrentSeconds: totalDuration,
					TotalSeconds:   totalDuration,
					Percentage:     100,
				},
			}) {
				return
			}
			EmitLog("success", fmt.Sprintf("视频《%s》已观看完毕！", videoTitle))
		}
		if planFailed {
			failed++
		} else {
			succeeded++
		}

		// 项目间冷却休眠 2 秒
		if idx < total-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}
