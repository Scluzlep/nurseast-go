package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"nurseast-go/pkg/api"
)

type LearningRecordModel struct {
	walk.TableModelBase
	walk.SorterBase
	items []api.LearningRecordItem
}

func (m *LearningRecordModel) Sort(col int, order walk.SortOrder) error {
	if col >= 0 && col <= 4 {
		sort.SliceStable(m.items, func(i, j int) bool {
			a, b := m.items[i], m.items[j]
			comparison := 0
			switch col {
			case 0:
				if a.Month < b.Month {
					comparison = -1
				} else if a.Month > b.Month {
					comparison = 1
				}
			case 1:
				comparison = strings.Compare(a.Name, b.Name)
			case 2:
				comparison = strings.Compare(a.Time, b.Time)
			case 3:
				comparison = strings.Compare(a.MainName, b.MainName)
			case 4:
				comparison = strings.Compare(a.ItemName, b.ItemName)
			}
			if order == walk.SortDescending {
				return comparison > 0
			}
			return comparison < 0
		})
	}
	return m.SorterBase.Sort(col, order)
}

func (m *LearningRecordModel) RowCount() int {
	return len(m.items)
}

func (m *LearningRecordModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	item := m.items[row]
	switch col {
	case 0:
		return fmt.Sprintf("%d 月", item.Month)
	case 1:
		return item.Name
	case 2:
		return item.Time
	case 3:
		return item.MainName
	case 4:
		return item.ItemName
	}
	return ""
}

func onShowLearningRecords() {
	token := apiClient.GetAccessToken()
	if token == "" {
		walk.MsgBox(mw, "提示", "请先验证 AccessToken 或完成登录。", walk.MsgBoxIconWarning)
		return
	}
	client := api.NewClient(token)
	model := &LearningRecordModel{}
	var dlg *walk.Dialog
	var cbType, cbYear *walk.ComboBox
	var tvRecords *walk.TableView
	var teCredit, teDetails *walk.TextEdit
	var lblCount *walk.Label
	var filters *api.LearningRecordFilters
	var filtersReady bool
	var recordRequestID, creditRequestID, detailRequestID uint64

	selectedFilters := func() (filterType, year int, ok bool) {
		if !filtersReady || filters == nil {
			return 0, 0, false
		}
		typeIndex, yearIndex := cbType.CurrentIndex(), cbYear.CurrentIndex()
		if typeIndex < 0 || typeIndex >= len(filters.FilterType) || yearIndex < 0 || yearIndex >= len(filters.FilterTime) {
			return 0, 0, false
		}
		return filters.FilterType[typeIndex].Value, filters.FilterTime[yearIndex].Value, true
	}

	loadRecords := func() {
		filterType, year, ok := selectedFilters()
		if !ok {
			return
		}
		recordRequestID++
		requestID := recordRequestID
		detailRequestID++
		model.items = nil
		model.PublishRowsReset()
		teDetails.SetText("请选择一条已学课程查看成绩明细。")
		lblCount.SetText("正在读取已学课程...")
		go func() {
			first, err := client.GetLearningRecords(filterType, year, 1)
			items := make([]api.LearningRecordItem, 0)
			if err == nil {
				items = append(items, first.Data...)
				pages := first.TotalPage
				if pages < 1 {
					pages = 1
				}
				for page := 2; page <= pages; page++ {
					data, pageErr := client.GetLearningRecords(filterType, year, page)
					if pageErr != nil {
						err = pageErr
						break
					}
					if data.Page != page {
						err = fmt.Errorf("服务端未返回第 %d 页", page)
						break
					}
					items = append(items, data.Data...)
				}
			}
			mw.Synchronize(func() {
				if dlg.IsDisposed() || requestID != recordRequestID {
					return
				}
				if err != nil {
					lblCount.SetText("读取课程失败: " + err.Error())
					return
				}
				model.items = items
				model.PublishRowsReset()
				_ = model.Sort(model.SortedColumn(), model.SortOrder())
				lblCount.SetText(fmt.Sprintf("共 %d 条已学课程", len(items)))
			})
		}()
	}

	loadCredit := func() {
		filterType, year, ok := selectedFilters()
		if !ok {
			return
		}
		creditRequestID++
		requestID := creditRequestID
		teCredit.SetText("正在计算学分...")
		go func() {
			data, err := client.GetLearningCredit(filterType, year)
			mw.Synchronize(func() {
				if dlg.IsDisposed() || requestID != creditRequestID {
					return
				}
				if err != nil {
					teCredit.SetText("读取学分失败: " + err.Error())
					return
				}
				lines := make([]string, 0, len(data.Credit)+len(data.Msg)+1)
				for _, item := range data.Credit {
					lines = append(lines, item.Name+": "+strconv.FormatFloat(item.Value, 'f', -1, 64))
				}
				if len(data.Msg) > 0 {
					lines = append(lines, "", "学分说明:")
					lines = append(lines, data.Msg...)
				}
				if len(lines) == 0 {
					lines = append(lines, "当前筛选条件下没有学分数据。")
				}
				teCredit.SetText(strings.Join(lines, "\r\n"))
			})
		}()
	}

	refresh := func() {
		if !filtersReady {
			return
		}
		loadCredit()
		loadRecords()
	}

	showDetails := func() {
		index := tvRecords.CurrentIndex()
		if index < 0 || index >= len(model.items) {
			return
		}
		item := model.items[index]
		detailRequestID++
		requestID := detailRequestID
		teDetails.SetText("正在读取课程成绩明细...")
		go func() {
			data, err := client.GetLearningPlanInfo(item.ID)
			mw.Synchronize(func() {
				if dlg.IsDisposed() || requestID != detailRequestID {
					return
				}
				if err != nil {
					teDetails.SetText("读取成绩明细失败: " + err.Error())
					return
				}
				lines := []string{item.Name}
				for _, metric := range data.Data {
					lines = append(lines, metric.Title+": "+fmt.Sprint(metric.Text))
				}
				if len(data.Data) == 0 {
					lines = append(lines, "暂无成绩明细。")
				}
				teDetails.SetText(strings.Join(lines, "\r\n"))
			})
		}()
	}

	view := Dialog{
		AssignTo: &dlg,
		Title:    "学分与已学课程",
		Icon:     appIcon,
		Font:     Font{Family: "Microsoft YaHei UI", PointSize: 11},
		MinSize:  Size{Width: 1000, Height: 760},
		Size:     Size{Width: 1100, Height: 860},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					Label{Text: "类别:"},
					ComboBox{AssignTo: &cbType, OnCurrentIndexChanged: refresh},
					Label{Text: "年份:"},
					ComboBox{AssignTo: &cbYear, OnCurrentIndexChanged: refresh},
					PushButton{Text: "刷新", OnClicked: refresh},
				},
			},
			GroupBox{
				Title:  "学分汇总",
				Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
				Children: []Widget{
					TextEdit{AssignTo: &teCredit, ReadOnly: true, VScroll: true, MinSize: Size{Height: 180}},
				},
			},
			Label{Text: "已学课程（选择一行查看成绩明细）:"},
			TableView{
				AssignTo: &tvRecords,
				Columns: []TableViewColumn{
					{Title: "月份", Width: 65},
					{Title: "名称", Width: 220},
					{Title: "时间", Width: 180},
					{Title: "分类", Width: 180},
					{Title: "项目", Width: 220},
				},
				Model:                 model,
				MinSize:               Size{Height: 260},
				OnCurrentIndexChanged: showDetails,
			},
			Label{AssignTo: &lblCount, Text: "正在读取筛选条件..."},
			GroupBox{
				Title:  "成绩明细",
				Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
				Children: []Widget{
					TextEdit{AssignTo: &teDetails, ReadOnly: true, VScroll: true, MinSize: Size{Height: 100}, Text: "请选择一条已学课程查看成绩明细。"},
				},
			},
		},
	}
	if err := view.Create(mw); err != nil {
		walk.MsgBox(mw, "错误", "无法打开学分窗口: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	go func() {
		data, err := client.GetLearningRecordFilters()
		mw.Synchronize(func() {
			if dlg.IsDisposed() {
				return
			}
			if err != nil {
				lblCount.SetText("读取筛选条件失败: " + err.Error())
				return
			}
			filters = data
			typeLabels := make([]string, len(data.FilterType))
			typeIndex := 0
			for i, option := range data.FilterType {
				typeLabels[i] = option.Label
				if option.Value == data.TypeValue {
					typeIndex = i
				}
			}
			yearLabels := make([]string, len(data.FilterTime))
			yearIndex := 0
			for i, option := range data.FilterTime {
				yearLabels[i] = strconv.Itoa(option.Label)
				if option.Value == data.TimeValue {
					yearIndex = i
				}
			}
			cbType.SetModel(typeLabels)
			cbYear.SetModel(yearLabels)
			cbType.SetCurrentIndex(typeIndex)
			cbYear.SetCurrentIndex(yearIndex)
			filtersReady = true
			refresh()
		})
	}()
	dlg.Run()
}
