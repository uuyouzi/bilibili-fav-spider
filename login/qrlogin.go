package login

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bili-downloader/api"
)

// QRLoginResult QR 登录成功后的结果
type QRLoginResult struct {
	Cookie string // 完整的 Cookie 字符串
}

// B站 QR 登录 API 响应结构

type qrGenerateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL       string `json:"url"`
		QrcodeKey string `json:"qrcode_key"`
	} `json:"data"`
}

type qrPollResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		URL     string `json:"url"`
	} `json:"data"`
}

// StartQRLogin 启动 QR 码登录流程
// 返回值包含登录后的 Cookie 和用户名
func StartQRLogin() (*QRLoginResult, error) {
	log.Println("========================================")
	log.Println("  未配置 Cookie，将启动扫码登录")
	log.Println("========================================")

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		// 1. 获取二维码
		qrcodeKey, loginURL, err := generateQRCode(client)
		if err != nil {
			return nil, fmt.Errorf("获取二维码失败: %w", err)
		}

		// 2. 在浏览器中展示二维码
		log.Println("正在打开浏览器展示二维码...")
		htmlPath, err := DisplayQRCode(loginURL)
		if err != nil {
			return nil, fmt.Errorf("展示二维码失败: %w", err)
		}
		log.Println("请在浏览器中扫描二维码登录 B站")

		// 3. 轮询扫码状态
		result, expired, err := pollQRCode(client, qrcodeKey)
		if err != nil {
			cleanupFile(htmlPath)
			return nil, fmt.Errorf("轮询登录状态失败: %w", err)
		}

		if expired {
			log.Println("二维码已过期，重新生成...")
			cleanupFile(htmlPath)
			continue
		}

		// 登录成功
		cleanupFile(htmlPath)
		log.Println("扫码登录成功！")
		return result, nil
	}
}

// generateQRCode 调用 B站 API 生成登录二维码
func generateQRCode(client *http.Client) (qrcodeKey, loginURL string, err error) {
	req, err := http.NewRequest("GET",
		"https://passport.bilibili.com/x/passport-login/web/qrcode/generate", nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求生成二维码失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取二维码响应失败: %w", err)
	}

	var result qrGenerateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("解析二维码响应失败: %w", err)
	}

	if result.Code != 0 {
		return "", "", fmt.Errorf("B站返回错误: %s (code=%d)", result.Message, result.Code)
	}

	return result.Data.QrcodeKey, result.Data.URL, nil
}

// pollQRCode 轮询 B站 API 检查扫码状态
// 返回：登录结果、是否过期、错误
func pollQRCode(client *http.Client, qrcodeKey string) (*QRLoginResult, bool, error) {
	log.Println("等待扫码...")

	// 最多轮询 5 分钟，避免异常情况下无限等待
	deadline := time.Now().Add(5 * time.Minute)

	for {
		if time.Now().After(deadline) {
			return nil, false, fmt.Errorf("扫码登录超时（5分钟），请重试")
		}

		time.Sleep(2 * time.Second)

		reqURL := fmt.Sprintf(
			"https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=%s",
			qrcodeKey,
		)

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, false, err
		}

		req.Header.Set("User-Agent", api.UserAgent)
		req.Header.Set("Referer", "https://www.bilibili.com")

		resp, err := client.Do(req)
		if err != nil {
			return nil, false, fmt.Errorf("轮询请求失败: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, false, fmt.Errorf("读取轮询响应失败: %w", err)
		}

		var result qrPollResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, false, fmt.Errorf("解析轮询响应失败: %w", err)
		}

		if result.Code != 0 {
			return nil, false, fmt.Errorf("B站返回错误: %s (code=%d)", result.Message, result.Code)
		}

		switch result.Data.Code {
		case 86101:
			// 未扫码，继续等待
			continue

		case 86090:
			// 已扫码，等待确认
			log.Println("已扫码，请在手机上确认登录...")
			continue

		case 86038:
			// 二维码过期
			return nil, true, nil

		case 0:
			// 登录成功，解析 Cookie
			cookie, err := parseLoginCookie(result.Data.URL)
			if err != nil {
				return nil, false, fmt.Errorf("解析登录 Cookie失败: %w", err)
			}

			return &QRLoginResult{Cookie: cookie}, false, nil

		default:
			log.Printf("未知状态码: %d, message=%s", result.Data.Code, result.Data.Message)
			continue
		}
	}
}

// parseLoginCookie 从登录成功回调 URL 中解析出 Cookie 字符串
// 回调 URL 格式: https://...?DedeUserID=xxx&SESSDATA=xxx&bili_jct=xxx&...
func parseLoginCookie(callbackURL string) (string, error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", fmt.Errorf("解析回调URL失败: %w", err)
	}

	query := u.Query()

	// 提取 Cookie 字段（排除 Expires 和 gourl 这两个非 Cookie 参数）
	var cookies []string
	cookieFields := []string{"DedeUserID", "DedeUserID__ckMd5", "SESSDATA", "bili_jct"}
	for _, field := range cookieFields {
		value := query.Get(field)
		if value == "" {
			continue
		}
		cookies = append(cookies, field+"="+value)
	}

	if len(cookies) == 0 {
		return "", fmt.Errorf("未从回调URL中找到Cookie字段")
	}

	return strings.Join(cookies, "; "), nil
}

