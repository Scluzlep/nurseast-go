package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	httpClient  *http.Client
	accessToken string
	mu          sync.RWMutex
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		accessToken: token,
	}
}

func (c *Client) SetAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token
}

func (c *Client) GetAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

func (c *Client) baseParams(method, apiVersion string) map[string]interface{} {
	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	callID := GenerateCallID()
	sig := GenerateSignature(DefaultAPIKey, callID, method)

	return map[string]interface{}{
		"sysVersion":   "12",
		"deviceModel":  "android",
		"codeVersion":  "7.6.9",
		"api_key":      DefaultAPIKey,
		"_app_project": "hlzs",
		"_project":     "hlzs",
		"method":       method,
		"apiversion":   apiVersion,
		"call_id":      callID,
		"sig":          sig,
		"access_token": token,
	}
}

func (c *Client) postJSON(url string, payload map[string]interface{}, out interface{}) error {
	return c.postJSONContext(context.Background(), url, payload, out)
}

func (c *Client) postJSONContext(ctx context.Context, url string, payload map[string]interface{}, out interface{}) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "okhttp/4.11.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求网络异常: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态异常: %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	return nil
}

func responseError(success bool, msg string) error {
	if success {
		return nil
	}
	if msg == "" {
		msg = "服务器返回业务失败"
	}
	return fmt.Errorf("请求失败: %s", msg)
}

// SendLoginCode 发送登录验证码
func (c *Client) SendLoginCode(phone string) (*LoginData, error) {
	url := "https://hlzs.hulizhushou.com/api/1.0/hlzs/login"
	payload := c.baseParams("hlzs/login", "1.0")
	payload["phone"] = phone

	type respWrapper struct {
		Success bool      `json:"success"`
		Data    LoginData `json:"data"`
		Msg     string    `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// SubmitLoginCode 校验验证码完成登录
func (c *Client) SubmitLoginCode(phone, code string) (*LoginData, error) {
	url := "https://hlzs.hulizhushou.com/api/1.0/hlzs/login"
	payload := c.baseParams("hlzs/login", "1.0")
	payload["phone"] = phone
	payload["re_code"] = code
	payload["device_token"] = ""
	payload["jpush_device_token"] = "190e35f7e1138e91ae8"
	payload["manufacturer_brand"] = "huawei"
	payload["manufacturer_token"] = ""
	payload["app_os"] = "android"

	type respWrapper struct {
		Success bool      `json:"success"`
		Data    LoginData `json:"data"`
		Msg     string    `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetSwitchUnitsInfo 获取所属单位、医院及对应的有效 access_token
func (c *Client) GetSwitchUnitsInfo() (*SwitchUnitsData, error) {
	url := "https://hlzs.hulizhushou.com/api/1.0/hlzs/user/switch_units_info"
	payload := c.baseParams("hlzs/user/switch_units_info", "1.0")
	payload["config_platform"] = "hlzs"
	payload["jpush_device_token"] = "190e35f7e1138e91ae8"
	payload["manufacturer_brand"] = "huawei"

	type innerData struct {
		Status string          `json:"status"`
		Data   SwitchUnitsData `json:"data"`
	}
	type respWrapper struct {
		Success bool      `json:"success"`
		Data    innerData `json:"data"`
		Msg     string    `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data.Data, nil
}

// GetUserInfo 获取当前登录用户的姓名等信息
func (c *Client) GetUserInfo() (*UserInfoData, error) {
	url := "https://hlzs.hulizhushou.com/api/2.0/hlzs/user_info"
	payload := c.baseParams("hlzs/user_info", "2.0")
	var resp struct {
		Success bool         `json:"success"`
		Data    UserInfoData `json:"data"`
		Msg     string       `json:"msg"`
	}
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetSubjects 获取培训大项科目
func (c *Client) GetSubjects() (*SubjectsData, error) {
	first, err := c.getSubjectsPage(1)
	if err != nil {
		return nil, err
	}
	for page := 2; page <= first.TotalPage; page++ {
		next, err := c.getSubjectsPage(page)
		if err != nil {
			return nil, err
		}
		if next.Page != page {
			return nil, fmt.Errorf("科目列表请求第 %d 页，服务端返回第 %d 页", page, next.Page)
		}
		first.Subjects = append(first.Subjects, next.Subjects...)
	}
	return first, nil
}

func (c *Client) getSubjectsPage(page int) (*SubjectsData, error) {
	url := "https://hlzs.hulizhushou.com/api/1.0/hlzs/pro/subjects"
	payload := c.baseParams("hlzs/pro/subjects", "1.0")
	payload["obj_type"] = 0
	payload["is_first"] = true
	payload["first_install"] = false
	if page > 1 {
		payload["page"] = page
	}

	type respWrapper struct {
		Success bool         `json:"success"`
		Data    SubjectsData `json:"data"`
		Msg     string       `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetPlans 获取指定科目下的具体任务计划列表
func (c *Client) GetPlans(itemID int64) (*PlansData, error) {
	url := "https://hlzs.hulizhushou.com/api/1.0/hlzs/pro/plans"
	payload := c.baseParams("hlzs/pro/plans", "1.0")
	payload["filterPlan"] = "-1"
	payload["is_first"] = true
	payload["use_page"] = false
	payload["item_id"] = itemID

	type respWrapper struct {
		Success bool      `json:"success"`
		Data    PlansData `json:"data"`
		Msg     string    `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSON(url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetVideoDetail 获取指定计划项目的视频元信息
func (c *Client) GetVideoDetail(projectID int64) (*VideoDetailData, error) {
	return c.GetVideoDetailContext(context.Background(), projectID)
}

func (c *Client) GetVideoDetailContext(ctx context.Context, projectID int64) (*VideoDetailData, error) {
	url := "https://www.hulizhushou.com/api/2.0/hlzs/video_study/detail"
	payload := c.baseParams("hlzs/video_study/detail", "2.0")
	payload["project_id"] = projectID

	type respWrapper struct {
		Success bool            `json:"success"`
		Data    VideoDetailData `json:"data"`
		Msg     string          `json:"msg"`
	}

	var resp respWrapper
	if err := c.postJSONContext(ctx, url, payload, &resp); err != nil {
		return nil, err
	}
	if err := responseError(resp.Success, resp.Msg); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// FlushVideo 模拟视频播放心跳与进度分段上报
func (c *Client) FlushVideo(projectID, vid int64, videoUserID interface{}, start, end int64) (bool, string, error) {
	return c.FlushVideoContext(context.Background(), projectID, vid, videoUserID, start, end)
}

func (c *Client) FlushVideoContext(ctx context.Context, projectID, vid int64, videoUserID interface{}, start, end int64) (bool, string, error) {
	url := "https://www.hulizhushou.com/api/3.0/hlzs/video_study/flush_video"
	payload := c.baseParams("hlzs/video_study/flush_video", "3.0")
	payload["id"] = projectID
	payload["vid"] = vid
	payload["video_user_id"] = videoUserID
	payload["last_et"] = end
	payload["rngs"] = []RangeItem{{S: start, E: end}}
	payload["rate"] = 1
	payload["flag"] = "save"

	type respWrapper struct {
		Success bool        `json:"success"`
		Msg     string      `json:"msg"`
		Data    interface{} `json:"data"`
	}

	var resp respWrapper
	if err := c.postJSONContext(ctx, url, payload, &resp); err != nil {
		return false, "", err
	}
	return resp.Success, resp.Msg, nil
}
