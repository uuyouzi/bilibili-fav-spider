package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"bili-downloader/models"
)

// Config 配置管理模块
// 负责从 config.yaml 文件中读取配置，并进行合法性校验

const (
	// ConfigFileName 配置文件名
	ConfigFileName = "config.yaml"
)

// Load 加载配置文件
// path: 配置文件路径，如果为空则使用当前目录下的 config.yaml
// 返回: 配置对象和错误信息
func Load(path string) (*models.Config, error) {
	// 如果路径为空，使用默认路径（当前工作目录下的 config.yaml）
	if path == "" {
		// 获取当前工作目录
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("无法获取当前工作目录: %w", err)
		}
		path = wd + string(os.PathSeparator) + ConfigFileName
	}

	// 读取配置文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("配置文件不存在或无法读取: %s\n请将 config.yaml 复制到程序同目录下", path)
	}

	// 解析 YAML 格式的配置文件
	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("配置文件格式错误（请检查是否为有效YAML）: %w", err)
	}

	// 校验配置项
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate 校验配置项的合法性
func validate(cfg *models.Config) error {
	// 登录方式校验
	if cfg.UID != "" {
		// 使用 UID 模式：抓取指定用户的公开收藏夹，无需登录
		log.Printf("使用 UID 模式，将抓取用户 %s 的公开收藏夹", cfg.UID)
	} else if cfg.Cookie == "" {
		log.Println("配置中未设置 Cookie 和 UID，将在启动时启动扫码登录")
	}

	// 检查保存路径是否设置
	if cfg.SavePath == "" {
		return fmt.Errorf("配置错误: save_path 不能为空")
	}

	// 检查起始日期格式
	_, err := parseDate(cfg.StartDate)
	if err != nil {
		return fmt.Errorf("配置错误: start_date 格式错误，请使用 YYYY-MM-DD 格式（如 2026-04-01）")
	}

	// 检查下载质量参数
	if cfg.DownloadQuality == "" {
		cfg.DownloadQuality = "bestvideo+bestaudio"
	}

	// 检查并发数
	if cfg.MaxConcurrentDownloads <= 0 {
		cfg.MaxConcurrentDownloads = 1
	}

	// 检查间隔时间
	if cfg.CheckIntervalMinutes <= 0 {
		cfg.CheckIntervalMinutes = 30
	}

	// 检查超时时间
	if cfg.DownloadTimeout <= 0 {
		cfg.DownloadTimeout = 3600
	}

	// 设置默认值：按收藏夹分类
	// FavoriteFolders 使用指针类型，nil 表示未设置，默认为 true
	if cfg.FavoriteFolders == nil {
		cfg.FavoriteFolders = models.BoolPtr(true)
	}

	// 检查下载模式
	if cfg.DownloadMode == "" {
		cfg.DownloadMode = "video"
	}
	if cfg.DownloadMode != "video" && cfg.DownloadMode != "metadata" {
		return fmt.Errorf("配置错误: download_mode 只能为 video 或 metadata")
	}

	return nil
}

// parseDate 解析日期字符串
// 格式: YYYY-MM-DD
func parseDate(dateStr string) (string, error) {
	if dateStr == "" {
		return "", fmt.Errorf("日期不能为空")
	}

	// 使用 time.Parse 严格校验日期合法性和格式
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return "", fmt.Errorf("日期格式错误，请使用 YYYY-MM-DD 格式（如 2026-04-01）")
	}

	return dateStr, nil
}

// EnsureSavePathExists 确保保存目录存在
// 如果目录不存在，会自动创建
func EnsureSavePathExists(savePath string) error {
	// os.MkdirAll 会递归创建目录
	// 第二个参数 0755 是目录权限：所有者可读写执行，其他人可读执行
	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("无法创建保存目录 %s: %w", savePath, err)
	}
	return nil
}
