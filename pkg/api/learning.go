package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type LearningFilterOption struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

type LearningYearOption struct {
	Value int `json:"value"`
	Label int `json:"label"`
}

type LearningRecordFilters struct {
	Status     string                 `json:"status"`
	FilterType []LearningFilterOption `json:"filter_type"`
	FilterTime []LearningYearOption   `json:"filter_time"`
	TypeValue  int                    `json:"type_value"`
	TimeValue  int                    `json:"time_value"`
}

type LearningCreditItem struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type LearningCredit struct {
	Status string               `json:"status"`
	Msg    []string             `json:"msg"`
	Credit []LearningCreditItem `json:"credit"`
}

type LearningRecordItem struct {
	ID        int64  `json:"id"`
	Month     int    `json:"month"`
	Name      string `json:"name"`
	Time      string `json:"time"`
	StartTime string `json:"start_time"`
	MainName  string `json:"main_name"`
	ItemName  string `json:"item_name"`
}

type LearningRecords struct {
	Status    string               `json:"status"`
	Page      int                  `json:"page"`
	TotalPage int                  `json:"total_page"`
	Data      []LearningRecordItem `json:"data"`
}

type LearningPlanMetric struct {
	Title string      `json:"title"`
	Text  interface{} `json:"text"`
}

type LearningPlanInfo struct {
	Status string               `json:"status"`
	Data   []LearningPlanMetric `json:"data"`
}

func (c *Client) getLearningData(method string, params map[string]interface{}, out interface{}) error {
	payload := c.baseParams(method, "1.0")
	for key, value := range params {
		payload[key] = value
	}
	var resp struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	url := "https://hlzs.hulizhushou.com/api/1.0/" + method
	if err := c.postJSON(url, payload, &resp); err != nil {
		return err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return err
	}
	if len(resp.Data) == 0 || bytes.Equal(resp.Data, []byte("null")) {
		return fmt.Errorf("接口未返回数据")
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return fmt.Errorf("解析学习记录状态失败: %w", err)
	}
	if status.Status != "" && status.Status != "success" {
		return fmt.Errorf("接口返回状态: %s", status.Status)
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return fmt.Errorf("解析学习记录失败: %w", err)
	}
	return nil
}

func (c *Client) GetLearningRecordFilters() (*LearningRecordFilters, error) {
	var result LearningRecordFilters
	if err := c.getLearningData("hlzs/user/learning_record_filter", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetLearningCredit(filterType, year int) (*LearningCredit, error) {
	var result LearningCredit
	params := map[string]interface{}{"filter_type": filterType, "filter_time": year}
	if err := c.getLearningData("hlzs/user/learning_record_credit", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetLearningRecords(filterType, year, page int) (*LearningRecords, error) {
	params := map[string]interface{}{"filter_type": filterType, "filter_time": year}
	if page > 1 {
		params["page"] = page
	}
	var result LearningRecords
	if err := c.getLearningData("hlzs/user/learning_record", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetLearningPlanInfo(id int64) (*LearningPlanInfo, error) {
	var result LearningPlanInfo
	if err := c.getLearningData("hlzs/user/learning_record_plan_info", map[string]interface{}{"id": id}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
