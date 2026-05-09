package models

import (
	"fmt"
	"time"
)

// VideoStatus 视频下载状态
// 共有4种状态：待下载、已下载、下载失败、已失效（被下架）
type VideoStatus string

const (
	StatusPending   VideoStatus = "pending"    // 待下载
	StatusDownloaded VideoStatus = "downloaded" // 已下载
	StatusFailed    VideoStatus = "failed"    // 下载失败
	StatusExpired   VideoStatus = "expired"   // 已失效（被下架）
)

// Video 视频数据模型
// 对应数据库中的 videos 表
type Video struct {
	ID            int64       `json:"id"`             // 数据库自增ID
	Bvid          string      `json:"bvid"`           // B站视频的唯一标识符（如 BV1xx4y1d7z9）
	Title         string      `json:"title"`          // 视频标题
	Desc         string      `json:"desc"`           // 视频简介
	CoverURL     string      `json:"cover_url"`     // 封面图 URL
	Author        string      `json:"author"`         // UP主名称
	AuthorMid     string      `json:"author_mid"`     // UP主mid
	Duration      int         `json:"duration"`       // 视频时长（秒）
	PubDate       time.Time   `json:"pub_date"`       // 视频发布时间
	FavoriteTime  time.Time   `json:"favorite_time"`  // 收藏时间
	FavoriteId    int64       `json:"favorite_id"`    // 收藏夹ID（用于区分不同收藏夹）
	FavoriteTitle string      `json:"favorite_title"` // 收藏夹名称（用于目录分类）
	Status        VideoStatus `json:"status"`         // 下载状态
	SavePath      string      `json:"save_path"`      // 本地保存路径（下载成功后填写）
	ErrorMsg      string      `json:"error_msg"`      // 失败原因（下载失败时填写）
	Retries       int         `json:"retries"`        // 重试次数
	CreatedAt     time.Time   `json:"created_at"`     // 记录创建时间
	UpdatedAt     time.Time   `json:"updated_at"`     // 记录更新时间
}

// String 返回视频的可读描述，用于日志输出
func (v *Video) String() string {
	return fmt.Sprintf("[%s] %s（%s）", v.Bvid, v.Title, v.Status)
}

// IsDownloadable 判断视频是否可以下载
// 只有未下载且未失效的视频才可以下载
func (v *Video) IsDownloadable() bool {
	return v.Status == StatusPending || v.Status == StatusFailed
}

// MarkAsDownloaded 标记视频为已下载状态
func (v *Video) MarkAsDownloaded(savePath string) {
	v.Status = StatusDownloaded
	v.SavePath = savePath
	v.UpdatedAt = time.Now()
}

// MarkAsFailed 标记视频为下载失败状态，并记录错误信息
func (v *Video) MarkAsFailed(errMsg string) {
	v.Status = StatusFailed
	v.ErrorMsg = errMsg
	v.UpdatedAt = time.Now()
}

// MarkAsExpired 标记视频为已失效状态（被下架）
func (v *Video) MarkAsExpired() {
	v.Status = StatusExpired
	v.UpdatedAt = time.Now()
}

// Favorite 收藏夹信息
// 用于配置要监控哪些收藏夹
type Favorite struct {
	ID    int64  `json:"id"`    // 收藏夹ID
	Title string `json:"title"` // 收藏夹名称
}

// Config 应用配置数据模型
// 用于从 config.yaml 文件中读取配置
type Config struct {
	// Cookie B站登录后的 Cookie（可选，不填则启动时自动扫码登录）
	// 如果同时设置了 uid，则使用 uid 模式（抓取公开收藏夹，无需登录）
	Cookie string `yaml:"cookie"`

	// UID 目标用户的 B站 UID（可选，填了就不需要 Cookie）
	// 抓取该用户公开可见的收藏夹，无需登录
	UID string `yaml:"uid"`

	// SavePath 视频保存路径
	// 程序会在此路径下创建子文件夹按 收藏夹/UP主 分类存储视频
	SavePath string `yaml:"save_path"`

	// StartDate 起始日期
	// 只监控此日期之后收藏的视频，之前收藏的会被跳过（标记为隔离）
	// 格式：2006-01-02（年月日）
	StartDate string `yaml:"start_date"`

	// CheckIntervalMinutes 检查间隔（分钟）
	// 程序每隔多长时间检查一次收藏夹
	// 建议至少30分钟，太频繁可能被B站风控
	CheckIntervalMinutes int `yaml:"check_interval_minutes"`

	// MaxConcurrentDownloads 最大并发下载数
	// 同时下载的视频数量，建议1-2，避免带宽占满
	MaxConcurrentDownloads int `yaml:"max_concurrent_downloads"`

	// DownloadQuality 下载质量
	// bestvideo+bestaudio = 最高画质（含音视频分离下载，最清晰）
	// best = 整体最清晰（可能画质稍低但文件单一）
	DownloadQuality string `yaml:"download_quality"`

	// EnableNotification 是否启用通知
	// 启用后下载成功/失败会打印到控制台
	EnableNotification bool `yaml:"enable_notification"`

	// DownloadTimeout 下载超时时间（秒）
	// 单个视频下载超过此时间会被判定为失败
	DownloadTimeout int `yaml:"download_timeout_seconds"`

	// FavoriteFolders 按收藏夹分类目录
	// true = 按 收藏夹名称/UP主/视频标题 保存
	// false = 按 UP主/视频标题 保存（不区分收藏夹）
	// 使用指针类型区分"未设置"和"显式设为false"
	FavoriteFolders *bool `yaml:"favorite_folders"`

	// MonitoredFavorites 要监控的收藏夹列表
	// 如果为空，则监控所有收藏夹
	// 可以只指定部分收藏夹，例如：["默认收藏夹", "AI"]
	MonitoredFavorites []string `yaml:"monitored_favorites"`

	// DownloadMode 下载模式
	// "video" = 下载完整视频（默认，需要 yt-dlp）
	// "metadata" = 仅保存标题和描述到 txt 文件（无需 yt-dlp）
	DownloadMode string `yaml:"download_mode"`
}

// IsFavoriteFolders 返回是否按收藏夹分类目录
// 如果未设置（nil），默认为 true
func (c *Config) IsFavoriteFolders() bool {
	if c.FavoriteFolders == nil {
		return true // 默认开启
	}
	return *c.FavoriteFolders
}

// BoolPtr 返回 bool 值的指针，用于 YAML 反序列化时区分零值和未设置
func BoolPtr(b bool) *bool {
	return &b
}

// DefaultConfig 返回默认配置
// 当用户没有提供 config.yaml 时使用这些默认值
func DefaultConfig() *Config {
	return &Config{
		Cookie:                   "",
		SavePath:                 "./downloads",
		StartDate:                "2000-01-01",
		CheckIntervalMinutes:     30,
		MaxConcurrentDownloads:   1,
		DownloadQuality:          "bestvideo+bestaudio",
		EnableNotification:       true,
		DownloadTimeout:          3600, // 默认1小时超时
		FavoriteFolders:         BoolPtr(true), // 默认按收藏夹分类
		MonitoredFavorites:       []string{},   // 空表示监控所有收藏夹
		DownloadMode:             "video",       // 默认下载完整视频
	}
}
