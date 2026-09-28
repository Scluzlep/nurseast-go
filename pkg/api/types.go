package api

import (
	"encoding/json"
	"strconv"
)

type NumberOrString float64

func (n *NumberOrString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = 0
		return nil
	}
	var value float64
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		if text == "" {
			*n = 0
			return nil
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return err
		}
		value = parsed
	} else if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*n = NumberOrString(value)
	return nil
}

// BaseResponse 统一响应外层结构
type BaseResponse struct {
	Success bool        `json:"success"`
	Time    string      `json:"time"`
	Auth    bool        `json:"auth"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
}

// LoginData 登录步骤返回的数据
type LoginData struct {
	Status      string `json:"status"`
	Extra       string `json:"extra"`
	ErrorCode   int    `json:"error_code"`
	UserID      int64  `json:"user_id"`
	AccessToken string `json:"access_token"`
}

// UnitItem 医院单位与 Token
type UnitItem struct {
	UnitID      int64  `json:"unit_id"`
	UnitName    string `json:"unit_name"`
	UnitType    string `json:"unit_type"`
	AccessToken string `json:"access_token"`
}

// SwitchUnitsData 单位切换信息
type SwitchUnitsData struct {
	UnitName    string     `json:"unit_name"`
	SelectType  string     `json:"select_type"`
	CurrentUnit UnitItem   `json:"current_unit"`
	Units       []UnitItem `json:"units"`
}

// UserInfoData 当前登录用户信息
type UserInfoData struct {
	Status string `json:"status"`
	Info   struct {
		Name string `json:"name"`
	} `json:"info"`
}

// SubjectItem 培训科目/大类
type SubjectItem struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	SubTitle1     string `json:"sub_title1"`
	SubTitle2     string `json:"sub_title2"`
	Badge         int    `json:"badge"`
	OrderTime     string `json:"order_time"`
	CountdownTime string `json:"countdown_time"`
	PlanTitle     string `json:"plan_title"`
	IsEnd         bool   `json:"is_end"`
	TargetType    string `json:"target_type"`
}

// SubjectsData 科目列表响应
type SubjectsData struct {
	Status    string        `json:"status"`
	Page      int           `json:"page"`
	TotalPage int           `json:"total_page"`
	Title     string        `json:"title"`
	Subjects  []SubjectItem `json:"subjects"`
}

// PlanItem 具体课程任务/计划 (这里的 ID 即为 project_id)
type PlanItem struct {
	ID                 int64  `json:"id"`
	Title              string `json:"title"`
	SubTitle1          string `json:"sub_title1"`
	SubTitle2          string `json:"sub_title2"`
	STime              string `json:"s_time"`
	ETime              string `json:"e_time"`
	IsDabiao           bool   `json:"is_dabiao"`
	RunningStatus      int    `json:"running_status"`
	RunningStatusColor string `json:"running_status_color"`
	URL                string `json:"url"`
}

// PlansData 计划任务列表
type PlansData struct {
	Status     string     `json:"status"`
	Plans      []PlanItem `json:"plans"`
	Page       int        `json:"page"`
	TotalPages int        `json:"total_pages"`
}

// VideoInfoItem 视频元数据
type VideoInfoItem struct {
	VID                 int64          `json:"vid"`
	Title               string         `json:"title"`
	VideoDurationSecond int64          `json:"video_duration_second"`
	LastTimePoint       NumberOrString `json:"last_time_point"`
	UseTime             string         `json:"use_time"`
}

// VideoDetailData 视频详情数据
type VideoDetailData struct {
	VideoDetails []VideoInfoItem        `json:"video_details"`
	VideoUserID  map[string]interface{} `json:"video_user_id"`
}

// RangeItem 观看时间段范围
type RangeItem struct {
	S int64 `json:"s"`
	E int64 `json:"e"`
}
