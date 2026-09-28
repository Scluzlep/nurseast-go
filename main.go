package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"nurseast-go/pkg/api"
	"nurseast-go/pkg/config"
	"nurseast-go/pkg/engine"
)

// PlanItemRow 表格行数据
type PlanItemRow struct {
	Checked   bool
	ID        int64
	Title     string
	Status    string
	IsDabiao  bool
	TimeRange string
}

// PlanModel 任务表格数据源
type PlanModel struct {
	walk.TableModelBase
	walk.SorterBase
	items []*PlanItemRow
}

func (m *PlanModel) RowCount() int {
	return len(m.items)
}

func (m *PlanModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	item := m.items[row]
	switch col {
	case 0:
		return item.Title
	case 1:
		return item.Status
	case 2:
		return item.TimeRange
	}
	return ""
}

func (m *PlanModel) Checked(row int) bool {
	if row < 0 || row >= len(m.items) {
		return false
	}
	return m.items[row].Checked
}

func (m *PlanModel) SetChecked(row int, checked bool) error {
	if row >= 0 && row < len(m.items) {
		m.items[row].Checked = checked
		if lblSelectedCount != nil {
			updateSelectedCountDisplay()
		}
	}
	return nil
}

func (m *PlanModel) Sort(col int, order walk.SortOrder) error {
	if col >= 0 && col <= 2 {
		sort.SliceStable(m.items, func(i, j int) bool {
			a, b := m.items[i], m.items[j]
			var left, right string
			switch col {
			case 0:
				left, right = a.Title, b.Title
			case 1:
				left, right = a.Status, b.Status
			case 2:
				left, right = a.TimeRange, b.TimeRange
			}
			comparison := strings.Compare(left, right)
			if order == walk.SortDescending {
				return comparison > 0
			}
			return comparison < 0
		})
	}
	return m.SorterBase.Sort(col, order)
}

// SubjectOption 科目下拉项
type SubjectOption struct {
	ID    int64
	Title string
}

var (
	appConfig *config.Config
	apiClient *api.Client
	appIcon   *walk.Icon

	mw               *walk.MainWindow
	teToken          *walk.LineEdit
	lblUnitStatus    *walk.Label
	cbSubjects       *walk.ComboBox
	tvPlans          *walk.TableView
	planModel        *PlanModel
	lblSelectedCount *walk.Label

	btnStart  *walk.PushButton
	btnPause  *walk.PushButton
	btnResume *walk.PushButton
	btnStop   *walk.PushButton

	lblOverallProgress *walk.Label
	pbOverall          *walk.ProgressBar
	lblVideoProgress   *walk.Label
	pbVideo            *walk.ProgressBar
	teLogs             *walk.TextEdit

	subjectsList   []api.SubjectItem
	subjectsMap    map[int]int64 // ComboBox index -> Subject ID
	credentialSeq  uint64
	subjectLoadSeq uint64
	planLoadSeq    uint64
)

func main() {
	appConfig = config.Load()
	apiClient = api.NewClient(appConfig.AccessToken)
	planModel = &PlanModel{items: make([]*PlanItemRow, 0)}
	subjectsMap = make(map[int]int64)

	if ico, err := walk.NewIconFromResourceId(100); err == nil {
		appIcon = ico
	} else if ico, err := walk.NewIconFromFile("logo.ico"); err == nil {
		appIcon = ico
	}

	MainWindow{
		AssignTo: &mw,
		Title:    "护理助手 - 刷课助手",
		Icon:     appIcon,
		Font:     Font{Family: "Microsoft YaHei UI", PointSize: 11},
		MinSize:  Size{Width: 900, Height: 800},
		Size:     Size{Width: 1020, Height: 880},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			// 标题说明
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{
						Text: "护理助手自动刷课助手",
						Font: Font{PointSize: 13, Bold: true},
					},
				},
			},

			// 第一步：账号凭证
			GroupBox{
				Title:  "【第一步】连接我的护理助手账号",
				Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
				Children: []Widget{
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "AccessToken 凭证:"},
							LineEdit{
								AssignTo: &teToken,
							},
							PushButton{
								Text:      "验证凭证 & 获取课程",
								OnClicked: onCheckToken,
							},
							PushButton{
								Text:      "微信验证码登录...",
								OnClicked: onShowLoginDialog,
							},
						},
					},
					Label{
						AssignTo:  &lblUnitStatus,
						Text:      getUnitDisplay(appConfig.UnitName, ""),
						TextColor: walk.RGB(0, 128, 0),
					},
				},
			},

			// 第二步：选择科目
			GroupBox{
				Title:  "【第二步】选择培训大课与待学清单",
				Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
				Children: []Widget{
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "请选择科目:"},
							ComboBox{
								AssignTo:              &cbSubjects,
								OnCurrentIndexChanged: onSubjectChanged,
							},
							PushButton{
								Text:      "刷新科目列表",
								OnClicked: loadSubjects,
							},
							PushButton{
								Text:      "学分与已学课程",
								OnClicked: onShowLearningRecords,
							},
						},
					},
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							PushButton{
								Text:      "一键勾选全部未学完",
								OnClicked: onSelectUnfinished,
							},
							PushButton{
								Text:      "全选 / 反选",
								OnClicked: onToggleSelectAll,
							},
							Label{
								AssignTo:  &lblSelectedCount,
								Text:      "已选中待学课程: 0 个",
								TextColor: walk.RGB(0, 70, 180),
							},
						},
					},
					TableView{
						AssignTo:   &tvPlans,
						CheckBoxes: true,
						Columns: []TableViewColumn{
							{Title: "课程任务名称", Width: 380},
							{Title: "当前状态", Width: 100},
							{Title: "培训时间", Width: 180},
						},
						Model: planModel,
					},
				},
			},

			// 第三步：刷课控制与进度
			GroupBox{
				Title:  "【第三步】开始自动刷课",
				Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 6},
				Children: []Widget{
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 10},
						Children: []Widget{
							PushButton{
								AssignTo:  &btnStart,
								Text:      "开始自动刷课",
								OnClicked: onStartTask,
							},
							PushButton{
								AssignTo:  &btnPause,
								Text:      "暂停",
								Enabled:   false,
								OnClicked: onPauseTask,
							},
							PushButton{
								AssignTo:  &btnResume,
								Text:      "继续",
								Enabled:   false,
								OnClicked: onResumeTask,
							},
							PushButton{
								AssignTo:  &btnStop,
								Text:      "终止任务",
								Enabled:   false,
								OnClicked: onStopTask,
							},
							PushButton{
								Text: "清空记录",
								OnClicked: func() {
									if teLogs != nil {
										teLogs.SetText("")
									}
								},
							},
							PushButton{
								Text:      "导出记录",
								OnClicked: onExportLogs,
							},
						},
					},
					Label{
						AssignTo: &lblOverallProgress,
						Text:     "总任务进度: 待命中",
					},
					ProgressBar{
						AssignTo: &pbOverall,
						MaxValue: 100,
					},
					Label{
						AssignTo: &lblVideoProgress,
						Text:     "当前视频进度: -",
					},
					ProgressBar{
						AssignTo: &pbVideo,
						MaxValue: 100,
					},
					Label{
						Text: "运行记录:",
					},
					TextEdit{
						AssignTo: &teLogs,
						ReadOnly: true,
						VScroll:  true,
					},
				},
			},
		},
	}.Create()

	// 启动后台事件监听更新原生控件
	go listenEngineEvents()

	if appConfig.AccessToken != "" {
		teToken.SetText(appConfig.AccessToken)
		appendLog("正在验证已保存的登录凭证...")
		onCheckToken()
	} else {
		appendLog("系统已就绪，请先在【第一步】输入 AccessToken 或点击微信登录。")
	}

	mw.Run()
}

func getUnitDisplay(unitName, userName string) string {
	if unitName == "" {
		return "单位归属: (未连接，请先点击【验证凭证】)"
	}
	if userName != "" {
		return fmt.Sprintf("单位归属: %s　姓名: %s (已连接)", unitName, userName)
	}
	return fmt.Sprintf("单位归属: %s (已连接)", unitName)
}

func onExportLogs() {
	if teLogs == nil || strings.TrimSpace(teLogs.Text()) == "" {
		walk.MsgBox(mw, "提示", "当前没有可导出的运行记录。", walk.MsgBoxIconInformation)
		return
	}
	dlg := walk.FileDialog{
		Title:       "导出运行记录",
		FilePath:    "运行记录-" + time.Now().Format("20060102-150405") + ".txt",
		Filter:      "文本文件 (*.txt)|*.txt",
		FilterIndex: 1,
		Flags:       win.OFN_OVERWRITEPROMPT,
	}
	accepted, err := dlg.ShowSave(mw)
	if err != nil {
		walk.MsgBox(mw, "导出失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	if !accepted {
		return
	}
	if err := os.WriteFile(dlg.FilePath, []byte(teLogs.Text()), 0644); err != nil {
		walk.MsgBox(mw, "导出失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	appendLog("运行记录已导出到: " + dlg.FilePath)
}

func appendLog(msg string) {
	if mw == nil {
		return
	}
	mw.Synchronize(func() {
		if teLogs == nil {
			return
		}
		t := time.Now().Format("15:04:05")
		current := teLogs.Text()
		newLine := fmt.Sprintf("[%s] %s\r\n", t, msg)
		teLogs.SetText(current + newLine)
		teLogs.SetTextSelection(len(teLogs.Text()), len(teLogs.Text()))
	})
}

func onCheckToken() {
	state := engine.GlobalEngine.GetState()
	if state == engine.StateRunning || state == engine.StatePaused || state == engine.StateStopping {
		walk.MsgBox(mw, "提示", "请先结束当前任务，再切换账号。", walk.MsgBoxIconWarning)
		return
	}
	token := strings.TrimSpace(teToken.Text())
	if token == "" {
		walk.MsgBox(mw, "提示", "请输入 AccessToken！", walk.MsgBoxIconWarning)
		return
	}

	credentialSeq++
	requestID := credentialSeq
	appendLog("正在连接服务器校验 Token 有效性...")
	go func() {
		client := api.NewClient(token)
		unitData, err := client.GetSwitchUnitsInfo()
		if err != nil {
			mw.Synchronize(func() {
				if requestID != credentialSeq {
					return
				}
				appendLog("凭证校验失败: " + err.Error())
				walk.MsgBox(mw, "错误", "Token 无效或已过期: "+err.Error(), walk.MsgBoxIconError)
			})
			return
		}

		unitName := unitData.UnitName
		if unitName == "" && unitData.CurrentUnit.UnitName != "" {
			unitName = unitData.CurrentUnit.UnitName
		}

		mw.Synchronize(func() {
			if requestID != credentialSeq {
				return
			}
			state := engine.GlobalEngine.GetState()
			if state == engine.StateRunning || state == engine.StatePaused || state == engine.StateStopping {
				appendLog("任务运行期间不切换账号，请任务结束后重新验证凭证。")
				return
			}
			apiClient.SetAccessToken(token)
			appConfig.AccessToken = token
			appConfig.UnitName = unitName
			clearCourseSelection()
			if err := config.Save(appConfig); err != nil {
				appendLog("凭证已验证，但保存配置失败: " + err.Error())
			}
			lblUnitStatus.SetText(getUnitDisplay(unitName, ""))
			appendLog(fmt.Sprintf("验证成功！当前所属单位: %s", unitName))
			loadSubjects()
		})
		loadUserName(client, token, unitName, requestID)
	}()
}

func clearCourseSelection() {
	subjectLoadSeq++
	planLoadSeq++
	subjectsList = nil
	subjectsMap = make(map[int]int64)
	cbSubjects.SetModel(nil)
	planModel.items = nil
	planModel.PublishRowsReset()
	updateSelectedCountDisplay()
	lblOverallProgress.SetText("总任务进度: 待命中")
	pbOverall.SetValue(0)
	lblVideoProgress.SetText("当前视频进度: -")
	pbVideo.SetValue(0)
}

func loadSubjects() {
	subjectLoadSeq++
	requestID := subjectLoadSeq
	token := apiClient.GetAccessToken()
	if token == "" {
		appendLog("请先验证 AccessToken，再刷新科目。")
		return
	}
	appendLog("正在从服务器拉取培训科目大类...")
	go func() {
		data, err := api.NewClient(token).GetSubjects()
		mw.Synchronize(func() {
			if requestID != subjectLoadSeq || token != apiClient.GetAccessToken() {
				return
			}
			if err != nil {
				appendLog("拉取科目失败: " + err.Error())
				return
			}

			subjectsList = data.Subjects
			subjectsMap = make(map[int]int64)
			items := []string{"-- 请点击下拉选择要学习的科目 --"}
			for idx, subj := range subjectsList {
				label := subj.Title
				if subj.Badge > 0 {
					label = fmt.Sprintf("%s (待学: %d)", label, subj.Badge)
				}
				items = append(items, label)
				subjectsMap[idx+1] = subj.ID
			}
			cbSubjects.SetModel(items)
			cbSubjects.SetCurrentIndex(0)
			appendLog(fmt.Sprintf("已成功载入 %d 个培训大课科目", len(subjectsList)))
		})
	}()
}

func onSubjectChanged() {
	planLoadSeq++
	requestID := planLoadSeq
	planModel.items = nil
	planModel.PublishRowsReset()
	updateSelectedCountDisplay()
	idx := cbSubjects.CurrentIndex()
	if idx <= 0 {
		return
	}
	subjectID, ok := subjectsMap[idx]
	if !ok {
		return
	}

	token := apiClient.GetAccessToken()
	appendLog(fmt.Sprintf("正在拉取所选科目 (ID: %d) 下的具体课程任务...", subjectID))
	go func() {
		data, err := api.NewClient(token).GetPlans(subjectID)
		if err != nil {
			mw.Synchronize(func() {
				if requestID == planLoadSeq {
					appendLog("获取课程任务清单失败: " + err.Error())
				}
			})
			return
		}

		rows := make([]*PlanItemRow, 0, len(data.Plans))
		for _, plan := range data.Plans {
			statusText := "未学完"
			if plan.IsDabiao {
				statusText = "已达标"
			}
			rows = append(rows, &PlanItemRow{
				Checked:   !plan.IsDabiao, // 默认只自动勾选未学完的
				ID:        plan.ID,
				Title:     plan.Title,
				Status:    statusText,
				IsDabiao:  plan.IsDabiao,
				TimeRange: plan.SubTitle1,
			})
		}

		mw.Synchronize(func() {
			if requestID != planLoadSeq || token != apiClient.GetAccessToken() || cbSubjects.CurrentIndex() != idx {
				return
			}
			planModel.items = rows
			planModel.PublishRowsReset()
			_ = planModel.Sort(planModel.SortedColumn(), planModel.SortOrder())
			updateSelectedCountDisplay()
			appendLog(fmt.Sprintf("该科目下共有 %d 个课程节点，已自动为您预选未达标课程", len(rows)))
		})
	}()
}

func loadUserName(client *api.Client, token, unitName string, requestID uint64) {
	go func() {
		info, err := client.GetUserInfo()
		mw.Synchronize(func() {
			if requestID != credentialSeq || token != apiClient.GetAccessToken() {
				return
			}
			if err != nil {
				appendLog("获取姓名失败: " + err.Error())
				return
			}
			name := strings.TrimSpace(info.Info.Name)
			if name == "" {
				appendLog("用户信息未返回姓名。")
				return
			}
			lblUnitStatus.SetText(getUnitDisplay(unitName, name))
		})
	}()
}

func onSelectUnfinished() {
	for _, it := range planModel.items {
		it.Checked = !it.IsDabiao
	}
	planModel.PublishRowsReset()
	updateSelectedCountDisplay()
}

func onToggleSelectAll() {
	anyUnchecked := false
	for _, it := range planModel.items {
		if !it.Checked {
			anyUnchecked = true
			break
		}
	}
	for _, it := range planModel.items {
		it.Checked = anyUnchecked
	}
	planModel.PublishRowsReset()
	updateSelectedCountDisplay()
}

func updateSelectedCountDisplay() {
	count := 0
	for _, it := range planModel.items {
		if it.Checked {
			count++
		}
	}
	lblSelectedCount.SetText(fmt.Sprintf("已选中待学课程: %d 个", count))
}

func onStartTask() {
	var selectedIDs []int64
	for _, it := range planModel.items {
		if it.Checked {
			selectedIDs = append(selectedIDs, it.ID)
		}
	}

	if len(selectedIDs) == 0 {
		walk.MsgBox(mw, "提示", "请至少勾选一个待刷课程！", walk.MsgBoxIconWarning)
		return
	}

	appendLog(fmt.Sprintf("启动自动刷课流程，共勾选 %d 个项目", len(selectedIDs)))
	if err := engine.GlobalEngine.Start(selectedIDs, apiClient, appConfig); err != nil {
		walk.MsgBox(mw, "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	lblOverallProgress.SetText("总任务进度: 正在开始...")
	pbOverall.SetValue(0)
	lblVideoProgress.SetText("当前视频进度: -")
	pbVideo.SetValue(0)
}

func onPauseTask() {
	engine.GlobalEngine.Pause()
}

func onResumeTask() {
	engine.GlobalEngine.Resume()
}

func onStopTask() {
	engine.GlobalEngine.Stop()
}

func listenEngineEvents() {
	ch := engine.Hub.Subscribe()
	defer engine.Hub.Unsubscribe(ch)

	for msg := range ch {
		var event engine.Event
		// 广播给界面
		_ = event
		mw.Synchronize(func() {
			// 解析各类型
			handleEngineEvent(msg)
		})
	}
}

func handleEngineEvent(data []byte) {
	// 简单的事件快速更新
	s := string(data)
	if strings.Contains(s, `"type":"log"`) {
		var e struct {
			Data struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &e); err == nil {
			appendLog(e.Data.Message)
		}
	} else if strings.Contains(s, `"type":"progress"`) {
		var e struct {
			Data struct {
				Current      int    `json:"current"`
				Total        int    `json:"total"`
				Percentage   int    `json:"percentage"`
				CurrentTitle string `json:"current_title"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &e); err == nil {
			lblOverallProgress.SetText(fmt.Sprintf("总任务进度: %s (%d/%d, %d%%)",
				e.Data.CurrentTitle, e.Data.Current, e.Data.Total, e.Data.Percentage))
			pbOverall.SetValue(e.Data.Percentage)
		}
	} else if strings.Contains(s, `"type":"video_progress"`) {
		var e struct {
			Data struct {
				Title          string `json:"title"`
				CurrentSeconds int64  `json:"current_seconds"`
				TotalSeconds   int64  `json:"total_seconds"`
				Percentage     int    `json:"percentage"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &e); err == nil {
			lblVideoProgress.SetText(fmt.Sprintf("当前视频: 《%s》 %ds / %ds (%d%%)",
				e.Data.Title, e.Data.CurrentSeconds, e.Data.TotalSeconds, e.Data.Percentage))
			pbVideo.SetValue(e.Data.Percentage)
		}
	} else if strings.Contains(s, `"type":"status"`) {
		var e struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &e); err == nil {
			switch e.Data.Status {
			case "running":
				btnStart.SetEnabled(false)
				btnPause.SetEnabled(true)
				btnResume.SetEnabled(false)
				btnStop.SetEnabled(true)
			case "paused":
				btnStart.SetEnabled(false)
				btnPause.SetEnabled(false)
				btnResume.SetEnabled(true)
				btnStop.SetEnabled(true)
			case "stopping":
				btnStart.SetEnabled(false)
				btnPause.SetEnabled(false)
				btnResume.SetEnabled(false)
				btnStop.SetEnabled(false)
				lblOverallProgress.SetText("总任务进度: 正在终止...")
			case "stopped":
				btnStart.SetEnabled(true)
				btnPause.SetEnabled(false)
				btnResume.SetEnabled(false)
				btnStop.SetEnabled(false)
				lblOverallProgress.SetText("总任务进度: 已终止")
			default:
				btnStart.SetEnabled(true)
				btnPause.SetEnabled(false)
				btnResume.SetEnabled(false)
				btnStop.SetEnabled(false)
			}
		}
	}
}

// onShowLoginDialog 弹出手机号微信登录对话框
func onShowLoginDialog() {
	var dlg *walk.Dialog
	var tePhone, teCode *walk.LineEdit
	var btnSend *walk.PushButton

	Dialog{
		AssignTo: &dlg,
		Title:    "手机号/微信验证码登录",
		Icon:     appIcon,
		Font:     Font{Family: "Microsoft YaHei UI", PointSize: 11},
		MinSize:  Size{Width: 420, Height: 260},
		Layout:   VBox{Margins: Margins{Left: 16, Top: 16, Right: 16, Bottom: 16}, Spacing: 10},
		Children: []Widget{
			Label{Text: "手机号 (注册护理助手的手机号):"},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					LineEdit{AssignTo: &tePhone, Text: appConfig.Phone},
					PushButton{
						AssignTo: &btnSend,
						Text:     "发送微信验证码",
						OnClicked: func() {
							phone := strings.TrimSpace(tePhone.Text())
							if phone == "" {
								walk.MsgBox(dlg, "提示", "请输入手机号！", walk.MsgBoxIconWarning)
								return
							}
							btnSend.SetEnabled(false)
							appendLog("正在发送登录验证码...")
							go func() {
								data, err := apiClient.SendLoginCode(phone)
								dlg.Synchronize(func() {
									btnSend.SetEnabled(true)
									if err != nil {
										walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
									} else {
										msg := data.Extra
										if msg == "" {
											msg = "验证码已发送至微信公众号，请查收"
										}
										walk.MsgBox(dlg, "已发送", msg, walk.MsgBoxIconInformation)
									}
								})
							}()
						},
					},
				},
			},
			Label{Text: "微信推送的 6 位验证码:"},
			LineEdit{AssignTo: &teCode},
			PushButton{
				Text: "核验并自动保存登录",
				OnClicked: func() {
					phone := strings.TrimSpace(tePhone.Text())
					code := strings.TrimSpace(teCode.Text())
					if phone == "" || code == "" {
						walk.MsgBox(dlg, "提示", "请输入手机号和验证码！", walk.MsgBoxIconWarning)
						return
					}
					state := engine.GlobalEngine.GetState()
					if state == engine.StateRunning || state == engine.StatePaused || state == engine.StateStopping {
						walk.MsgBox(dlg, "提示", "请先结束当前任务，再切换账号。", walk.MsgBoxIconWarning)
						return
					}

					credentialSeq++
					requestID := credentialSeq
					appendLog("正在核验验证码登录...")
					go func() {
						loginClient := api.NewClient(apiClient.GetAccessToken())
						loginData, err := loginClient.SubmitLoginCode(phone, code)
						if err != nil {
							dlg.Synchronize(func() {
								if requestID == credentialSeq {
									walk.MsgBox(dlg, "登录失败", err.Error(), walk.MsgBoxIconError)
								}
							})
							return
						}
						if loginData.AccessToken != "" {
							loginClient.SetAccessToken(loginData.AccessToken)
						}

						unitData, err := loginClient.GetSwitchUnitsInfo()
						if err != nil {
							dlg.Synchronize(func() {
								if requestID == credentialSeq {
									walk.MsgBox(dlg, "错误", "获取单位失败: "+err.Error(), walk.MsgBoxIconError)
								}
							})
							return
						}

						token := unitData.CurrentUnit.AccessToken
						if token == "" && len(unitData.Units) > 0 {
							token = unitData.Units[0].AccessToken
						}
						if token == "" {
							token = loginData.AccessToken
						}
						if token == "" {
							dlg.Synchronize(func() {
								if requestID == credentialSeq {
									walk.MsgBox(dlg, "登录失败", "服务器未返回 AccessToken，无法继续。", walk.MsgBoxIconError)
								}
							})
							return
						}

						unitName := unitData.UnitName
						if unitName == "" {
							unitName = unitData.CurrentUnit.UnitName
						}

						mw.Synchronize(func() {
							if requestID != credentialSeq {
								return
							}
							state := engine.GlobalEngine.GetState()
							if state == engine.StateRunning || state == engine.StatePaused || state == engine.StateStopping {
								appendLog("任务运行期间不切换账号，请任务结束后重新登录。")
								return
							}
							apiClient.SetAccessToken(token)
							appConfig.AccessToken = token
							appConfig.Phone = phone
							appConfig.UnitName = unitName
							clearCourseSelection()
							if err := config.Save(appConfig); err != nil {
								appendLog("登录成功，但保存配置失败: " + err.Error())
							}
							teToken.SetText(token)
							lblUnitStatus.SetText(getUnitDisplay(unitName, ""))
							dlg.Accept()
							walk.MsgBox(mw, "成功", fmt.Sprintf("登录成功！\n所属单位: %s\n已自动保存 Token", unitName), walk.MsgBoxIconInformation)
							appendLog("微信登录成功！Token 已自动同步")
							loadSubjects()
						})
						loadUserName(api.NewClient(token), token, unitName, requestID)
					}()
				},
			},
		},
	}.Run(mw)
}
