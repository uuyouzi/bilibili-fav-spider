package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bili-downloader/models"
)

// BilibiliAPI B站 API 调用模块
// 负责与 B站服务器通信，获取收藏夹信息

const (
	// FavoriteListAPI 收藏夹列表 API（新版，支持一次获取所有收藏夹）
	// 旧版 API (medialist/v2/favorite) 已弃用，新版 API 更稳定且无需分页
	FavoriteListAPI = "https://api.bilibili.com/x/v3/fav/folder/created/list-all"

	// VideoDetailAPI 视频详情 API
	// 用于获取单个视频的详细信息（标题、UP主、时长等）
	VideoDetailAPI = "https://api.bilibili.com/x/web-interface/view"

	// UserAgent 模拟浏览器的 User-Agent
	// B站 API 会检查 User-Agent，太简单可能会被拒绝
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// BilibiliClient B站 API 客户端
type BilibiliClient struct {
	Cookie     string        // 登录 Cookie
	UID        string        // 目标用户 UID（可选，用于公开收藏夹）
	HTTPClient *http.Client  // HTTP 客户端
}

// NewBilibiliClient 创建 B站 API 客户端
func NewBilibiliClient(cookie, uid string) *BilibiliClient {
	return &BilibiliClient{
		Cookie: cookie,
		UID:    uid,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FavoriteListResponse 收藏夹列表 API 响应结构（新版 API）
// 对应 B站新版 API (x/v3/fav/folder/created/list-all) 返回的 JSON 格式
type FavoriteListResponse struct {
	Code    int    `json:"code"`    // 0=成功，其他=失败
	Message string `json:"message"` // 错误信息
	TTL     int    `json:"ttl"`    // TTL
	Data    struct {
		List []struct {
			Id         int64  `json:"id"`          // 收藏夹ID
			Title      string `json:"title"`        // 收藏夹标题
			MediaCount int    `json:"media_count"`  // 收藏夹内视频数量
		} `json:"list"` // 收藏夹列表
	} `json:"data"`
}

// FavoriteVideosResponse 收藏夹内视频列表 API 响应结构（新接口 x/v3/fav/resource/list）
type FavoriteVideosResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		HasMore bool `json:"has_more"`
		Medias  []struct {
			Id           int64  `json:"id"`
			Bvid         string `json:"bvid"`          // 视频BV号
			Title        string `json:"title"`         // 视频标题
			Intro        string `json:"intro"`         // 视频简介
			Duration     int    `json:"duration"`       // 视频时长（秒）
			PubTime      int64  `json:"pubtime"`       // 发布时间（Unix时间戳）
			FavoriteTime int64  `json:"fav_time"`      // 收藏时间（Unix时间戳）
			Upper        struct {
				Mid  int64  `json:"mid"`  // UP主mid
				Name string `json:"name"` // UP主名称
			} `json:"upper"`
			// attr 字段：标记视频状态
			// attr & 2 != 0 表示视频已失效（被删除/下架）
			Attr int `json:"attr"`
		} `json:"medias"`
	} `json:"data"`
}

// GetFavoriteList 获取用户的收藏夹列表（使用新版 API，一次获取全部）
// 返回收藏夹信息列表
func (c *BilibiliClient) GetFavoriteList() ([]struct {
	Id         int64
	Title      string
	MediaCount int
}, error) {
	// 获取用户 mid：优先使用配置的 UID，否则从登录态获取
	var mid int64
	if c.UID != "" {
		var err error
		mid, err = strconv.ParseInt(c.UID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("UID 格式错误: %w", err)
		}
	} else {
		var err error
		mid, err = c.getUserMid()
		if err != nil {
			return nil, fmt.Errorf("获取用户信息失败: %w", err)
		}
	}

	reqURL := fmt.Sprintf("%s?up_mid=%d", FavoriteListAPI, mid)

	// 发起 GET 请求
	resp, err := c.doRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("获取收藏夹列表失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取收藏夹列表响应失败: %w", err)
	}

	// 解析 JSON 响应
	var result FavoriteListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析收藏夹列表响应失败: %w", err)
	}

	// 检查 API 返回码
	if result.Code != 0 {
		return nil, fmt.Errorf("B站 API 返回错误: %s (code=%d)", result.Message, result.Code)
	}

	// 提取收藏夹信息
	var favorites []struct {
		Id         int64
		Title      string
		MediaCount int
	}
	for _, fav := range result.Data.List {
		favorites = append(favorites, struct {
			Id         int64
			Title      string
			MediaCount int
		}{
			Id:         fav.Id,
			Title:      fav.Title,
			MediaCount: fav.MediaCount,
		})
	}

	return favorites, nil
}

// getUserMid 从 nav API 获取当前登录用户的 mid
func (c *BilibiliClient) getUserMid() (int64, error) {
	resp, err := c.doRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return 0, fmt.Errorf("请求用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取用户信息响应失败: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Mid     int64  `json:"mid"`
			IsLogin bool   `json:"isLogin"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析用户信息失败: %w", err)
	}

	if result.Code != 0 || !result.Data.IsLogin {
		return 0, fmt.Errorf("Cookie 无效或未登录")
	}

	return result.Data.Mid, nil
}

// GetFavoriteVideos 获取指定收藏夹中的所有视频
// mediaId: 收藏夹ID
// favoriteTitle: 收藏夹名称（用于目录分类）
// 返回: 视频列表和错误信息
func (c *BilibiliClient) GetFavoriteVideos(mediaId int64, favoriteTitle string) ([]*models.Video, error) {
	var allVideos []*models.Video
	page := 1
	const pageSize = 20

	for {
		reqURL := fmt.Sprintf(
			"https://api.bilibili.com/x/v3/fav/resource/list?media_id=%d&pn=%d&ps=%d&platform=web",
			mediaId, page, pageSize,
		)

		// 带重试：B站限流时会返回非JSON，等几秒重试
		var result *FavoriteVideosResponse
		var lastErr error
		for retry := 0; retry < 3; retry++ {
			if retry > 0 {
				backoff := time.Duration(retry) * 3 * time.Second
				log.Printf("请求被限流，重试第 %d 次（等待 %v）...", retry, backoff)
				time.Sleep(backoff)
			}

			resp, err := c.doRequest("GET", reqURL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}

			var r FavoriteVideosResponse
			if err := json.Unmarshal(body, &r); err != nil {
				lastErr = fmt.Errorf("非JSON响应(HTTP %d)", resp.StatusCode)
				continue
			}

			if r.Code != 0 {
				return nil, fmt.Errorf("B站 API 返回错误: %s (code=%d)", r.Message, r.Code)
			}

			result = &r
			break
		}

		if result == nil {
			return nil, fmt.Errorf("请求失败(第%d页，重试3次): %w", page, lastErr)
		}

		for _, item := range result.Data.Medias {
			video := &models.Video{
				Bvid:          item.Bvid,
				Title:         item.Title,
				Desc:          item.Intro,
				Author:        item.Upper.Name,
				AuthorMid:     strconv.FormatInt(item.Upper.Mid, 10),
				Duration:      item.Duration,
				PubDate:       time.Unix(item.PubTime, 0),
				FavoriteTime:  time.Unix(item.FavoriteTime, 0),
				FavoriteId:    mediaId,
				FavoriteTitle: favoriteTitle,
				Status:        models.StatusPending,
				Retries:       0,
			}
			if item.Attr&2 != 0 {
				video.Status = models.StatusExpired
			}
			allVideos = append(allVideos, video)
		}

		log.Printf("已获取第 %d 页，共 %d 个视频...", page, len(allVideos))

		if !result.Data.HasMore {
			break
		}

		page++
		time.Sleep(1 * time.Second)
	}

	return allVideos, nil
}

// GetAllFavoritesVideos 获取用户所有收藏夹的所有视频
// monitoredFavorites: 要监控的收藏夹名称列表（空列表表示监控所有）
// 返回: 视频列表和错误信息
func (c *BilibiliClient) GetAllFavoritesVideos(monitoredFavorites []string) ([]*models.Video, error) {
	// 先获取收藏夹列表
	favorites, err := c.GetFavoriteList()
	if err != nil {
		return nil, err
	}

	if len(favorites) == 0 {
		return nil, fmt.Errorf("未找到任何收藏夹，请确认 Cookie 是否有效")
	}

	log.Printf("发现 %d 个收藏夹", len(favorites))

	// 如果指定了要监控的收藏夹，进行过滤
	var filteredFavorites []struct {
		Id         int64
		Title      string
		MediaCount int
	}

	if len(monitoredFavorites) > 0 {
		for _, fav := range favorites {
			for _, monitored := range monitoredFavorites {
				// 支持模糊匹配（收藏夹名称包含关键词即可）
				if strings.Contains(fav.Title, monitored) || strings.Contains(monitored, fav.Title) {
					filteredFavorites = append(filteredFavorites, fav)
					log.Printf("✓ 将监控收藏夹: %s", fav.Title)
					break
				}
			}
		}
		if len(filteredFavorites) == 0 {
			log.Printf("未找到匹配的收藏夹，将监控所有收藏夹")
			filteredFavorites = favorites
		}
	} else {
		// 没有指定过滤，则监控所有
		filteredFavorites = favorites
		for _, fav := range favorites {
			log.Printf("✓ 将监控收藏夹: %s", fav.Title)
		}
	}

	var allVideos []*models.Video

	// 遍历每个收藏夹
	for i, fav := range filteredFavorites {
		log.Printf("正在获取收藏夹 [%d/%d]: %s (%d个视频)",
			i+1, len(filteredFavorites), fav.Title, fav.MediaCount)

		// 获取该收藏夹的所有视频，传入收藏夹名称
		videos, err := c.GetFavoriteVideos(fav.Id, fav.Title)
		if err != nil {
			log.Printf("获取收藏夹 '%s' 失败: %v，跳过...", fav.Title, err)
			continue
		}

		allVideos = append(allVideos, videos...)

		// 添加延迟，避免请求过快
		if i < len(filteredFavorites)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return allVideos, nil
}

// GetVideoInfo 获取单个视频的详细信息
// bvid: 视频BV号
func (c *BilibiliClient) GetVideoInfo(bvid string) (*models.Video, error) {
	reqURL := fmt.Sprintf("%s?bvid=%s", VideoDetailAPI, bvid)

	resp, err := c.doRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Bvid      string `json:"bvid"`
			Title    string `json:"title"`
			Desc     string `json:"desc"`
			Duration int    `json:"duration"`
			PubDate  int64  `json:"pubdate"`
			Owner    struct {
				Mid  int64  `json:"mid"`
				Name string `json:"name"`
			} `json:"owner"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析视频信息失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("视频信息获取失败: %s", result.Message)
	}

	return &models.Video{
		Bvid:     result.Data.Bvid,
		Title:    result.Data.Title,
		Desc:     result.Data.Desc,
		Author:   result.Data.Owner.Name,
		AuthorMid: strconv.FormatInt(result.Data.Owner.Mid, 10),
		Duration: result.Data.Duration,
		PubDate:  time.Unix(result.Data.PubDate, 0),
	}, nil
}

// doRequest 发起 HTTP 请求
// method: GET/POST
// url: 请求URL
// body: 请求体（用于 POST）
func (c *BilibiliClient) doRequest(method, reqURL string, body io.Reader) (*http.Response, error) {
	// 创建请求
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cookie", c.Cookie)
	req.Header.Set("Referer", "https://www.bilibili.com") // B站 API 要求 Referer

	// 发起请求
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ValidateCookie 验证 Cookie 是否有效
// 通过调用一个简单的 API 来检测
func (c *BilibiliClient) ValidateCookie() error {
	// 使用获取用户信息 API 来验证 Cookie
	resp, err := c.doRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return fmt.Errorf("验证 Cookie 失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		TTL     int    `json:"ttl"`
		Data    struct {
			Uname    string `json:"uname"`    // 用户名
			IsLogin  bool   `json:"isLogin"`  // 是否登录
			Level    int    `json:"level"`     // 账号等级
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析验证响应失败: %w", err)
	}

	if result.Code != 0 || !result.Data.IsLogin {
		return fmt.Errorf("Cookie 无效或已过期，请重新获取")
	}

	log.Printf("Cookie 验证成功！登录账号: %s（Lv.%d）", result.Data.Uname, result.Data.Level)
	return nil
}

// BuildVideoURL 根据 bvid 构建视频播放页面 URL
func BuildVideoURL(bvid string) string {
	return fmt.Sprintf("https://www.bilibili.com/video/%s", bvid)
}

// ParseBvidFromURL 从 URL 中提取 bvid
// 支持以下格式：
// - https://www.bilibili.com/video/BV1xx4y1d7z9
// - BV1xx4y1d7z9
func ParseBvidFromURL(input string) (string, error) {
	// 如果输入已经是 BV 号，直接返回
	if strings.HasPrefix(input, "BV") && len(input) >= 12 {
		return input, nil
	}

	// 尝试从 URL 中提取
	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("无效的 URL 格式")
	}

	// 从路径中提取 bvid
	// 路径格式: /video/BV1xx4y1d7z9
	pathParts := strings.Split(u.Path, "/")
	for i, part := range pathParts {
		if part == "video" && i+1 < len(pathParts) {
			bvid := pathParts[i+1]
			if strings.HasPrefix(bvid, "BV") {
				return bvid, nil
			}
		}
	}

	return "", fmt.Errorf("无法从输入中提取有效的 B站视频 BV 号")
}
