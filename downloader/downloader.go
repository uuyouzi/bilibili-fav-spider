package downloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bili-downloader/models"
)

// Downloader 视频下载器模块
// 使用 yt-dlp 作为下载引擎

const (
	// YtDlpName yt-dlp 可执行文件名
	YtDlpName = "yt-dlp"

	// DefaultDownloadTimeout 默认下载超时时间（秒）
	DefaultDownloadTimeout = 3600
)

// Config 下载器配置
type Config struct {
	// SavePath 保存路径
	SavePath string

	// Quality 下载质量
	// bestvideo+bestaudio = 最高画质
	// best = 整体最清晰
	Quality string

	// Timeout 下载超时（秒）
	Timeout int

	// MaxRetries 最大重试次数
	MaxRetries int

	// EnableNotification 是否启用通知
	EnableNotification bool
}

// Downloader 下载器结构体
type Downloader struct {
	config  *Config
	tempDir string // Cookie 临时文件目录
}

// New 创建下载器实例
func New(cfg *Config) *Downloader {
	// 设置默认值
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultDownloadTimeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Quality == "" {
		cfg.Quality = "bestvideo+bestaudio"
	}

	// 创建临时文件目录，用于存放 Cookie 文件
	tempDir := filepath.Join(cfg.SavePath, ".tmp")
	os.MkdirAll(tempDir, 0700)

	// 清理上次残留的旧 Cookie 文件
	cleanupOldTempFiles(tempDir)

	return &Downloader{config: cfg, tempDir: tempDir}
}

// CheckYtDlp 检查 yt-dlp 是否已安装
func (d *Downloader) CheckYtDlp() error {
	// 尝试执行 yt-dlp --version
	// 如果未安装，会返回错误
	cmd := exec.Command(YtDlpName, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(`未找到 yt-dlp，请先安装:
Windows: winget install yt-dlp
或从 https://github.com/yt-dlp/yt-dlp/releases 下载
macOS: brew install yt-dlp
Linux: sudo apt install yt-dlp`)
	}

	// 获取版本号
	output, _ := exec.Command(YtDlpName, "--version").Output()
	log.Printf("yt-dlp 版本: %s", strings.TrimSpace(string(output)))

	return nil
}

// DownloadResult 下载结果
type DownloadResult struct {
	Success  bool   // 是否成功
	SavePath string // 保存路径（成功时）
	ErrorMsg string // 错误信息（失败时）
}

// Download 下载单个视频
// video: 视频信息
// cookie: B站 Cookie（用于下载需要登录的视频）
// favoriteFolders: 是否按收藏夹分类目录
func (d *Downloader) Download(video *models.Video, cookie string, favoriteFolders bool) *DownloadResult {
	// 构建视频 URL
	videoURL := fmt.Sprintf("https://www.bilibili.com/video/%s", video.Bvid)

	// 构建保存路径：{SavePath}/{收藏夹名称}/{视频标题}/
	var saveDir string
	titleDir := sanitizeFilename(video.Title)
	if favoriteFolders && video.FavoriteTitle != "" {
		saveDir = filepath.Join(d.config.SavePath, sanitizeFilename(video.FavoriteTitle), titleDir)
	} else {
		saveDir = filepath.Join(d.config.SavePath, titleDir)
	}
	saveDir = uniqueDirPath(saveDir)
	saveTemplate := filepath.Join(saveDir, "%(title)s.%(ext)s")

	// 确保保存目录存在
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return &DownloadResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("无法创建保存目录: %v", err),
		}
	}

	// 构建 yt-dlp 命令行参数
	args := []string{
		"--format", d.config.Quality, // 视频质量
		"-o", saveTemplate,          // 保存路径模板
		"--no-playlist",             // 不下载整个播放列表（只下载单个视频）
		"--no-warnings",             // 不显示警告
		"-v",                        // 显示详细输出
	}

	// 添加 Cookie（如果有）
	if cookie != "" {
		// 将 Cookie 写入临时文件，使用 --cookies 参数传递给 yt-dlp
		// 这比 --cookies-from-browser 更通用（不依赖浏览器类型和是否存在）
		cookieFile, err := d.writeCookieFile(cookie)
		if err != nil {
			log.Printf("警告: 写入 Cookie 文件失败: %v，将尝试不带 Cookie 下载", err)
		} else {
			args = append(args, "--cookies", cookieFile)
			// 记录临时文件路径，下载完成后清理
			defer os.Remove(cookieFile)
		}
	}

	// 添加视频 URL
	args = append(args, videoURL)

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.config.Timeout)*time.Second)
	defer cancel()

	// 创建命令
	cmd := exec.CommandContext(ctx, YtDlpName, args...)
	cmd.Stdout = os.Stdout // 将标准输出重定向到程序标准输出
	cmd.Stderr = os.Stderr

	log.Printf("开始下载: %s", video.Title)
	log.Printf("保存到: %s", saveDir)

	// 执行下载
	startTime := time.Now()
	err := cmd.Run()
	elapsed := time.Since(startTime)

	if err != nil {
		// 检查是否是超时
		if ctx.Err() == context.DeadlineExceeded {
			return &DownloadResult{
				Success:  false,
				ErrorMsg: fmt.Sprintf("下载超时（超过%d秒）", d.config.Timeout),
			}
		}

		return &DownloadResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("下载失败: %v", err),
		}
	}

	// 查找实际保存的文件
	actualPath, err := d.findDownloadedFile(saveDir, video.Title)
	if err != nil {
		return &DownloadResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("下载完成但找不到文件: %v", err),
		}
	}

	log.Printf("下载完成! 耗时: %v, 路径: %s", elapsed, actualPath)

	return &DownloadResult{
		Success:  true,
		SavePath: actualPath,
	}
}

// findDownloadedFile 查找下载的文件
// 因为 yt-dlp 会自动命名，我们需要找到实际保存的文件
func (d *Downloader) findDownloadedFile(dir, title string) (string, error) {
	// 读取目录中的文件
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	// 查找与标题匹配的文件
	sanitizedTitle := sanitizeFilename(title)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filename := file.Name()
		// 检查文件名是否包含标题
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		if strings.Contains(nameWithoutExt, sanitizedTitle) ||
			sanitizedTitleContains(nameWithoutExt, sanitizedTitle) {
			return filepath.Join(dir, filename), nil
		}
	}

	// 如果没找到，返回目录中最新修改的文件
	var latestFile string
	var latestTime time.Time
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, _ := file.Info()
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = filepath.Join(dir, file.Name())
		}
	}

	if latestFile != "" {
		return latestFile, nil
	}

	return "", fmt.Errorf("在目录 %s 中未找到下载的文件", dir)
}

// SaveMetadata 保存视频元数据和封面图到以视频标题命名的文件夹
func (d *Downloader) SaveMetadata(video *models.Video, favoriteFolders bool) *DownloadResult {
	// 目录结构：{savePath}/{收藏夹}/{视频标题}/
	titleDir := sanitizeFilename(video.Title)
	var saveDir string
	if favoriteFolders && video.FavoriteTitle != "" {
		saveDir = filepath.Join(d.config.SavePath, sanitizeFilename(video.FavoriteTitle), titleDir)
	} else {
		saveDir = filepath.Join(d.config.SavePath, titleDir)
	}
	saveDir = uniqueDirPath(saveDir)

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return &DownloadResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("无法创建保存目录: %v", err),
		}
	}

	// 1. 保存详情 txt
	txtPath := filepath.Join(saveDir, "详情.txt")
	content := fmt.Sprintf(`标题: %s
BV号: %s
UP主: %s (mid: %s)
收藏夹: %s
收藏时间: %s
发布时间: %s
时长: %s
封面: %s
链接: https://www.bilibili.com/video/%s
描述:
%s
`,
		video.Title,
		video.Bvid,
		video.Author, video.AuthorMid,
		video.FavoriteTitle,
		video.FavoriteTime.Format("2006-01-02 15:04:05"),
		video.PubDate.Format("2006-01-02 15:04:05"),
		FormatDuration(video.Duration),
		video.CoverURL,
		video.Bvid,
		video.Desc,
	)

	if err := os.WriteFile(txtPath, []byte(content), 0644); err != nil {
		return &DownloadResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("写入详情文件失败: %v", err),
		}
	}

	// 2. 下载封面图
	if video.CoverURL != "" {
		if err := downloadCover(video.CoverURL, filepath.Join(saveDir, "封面.jpg")); err != nil {
			log.Printf("下载封面失败: %v", err)
		}
	}

	log.Printf("元数据已保存: %s", saveDir)
	return &DownloadResult{
		Success:  true,
		SavePath: saveDir,
	}
}

// downloadCover 下载封面图片到本地
// 带超时控制、Content-Type 校验和最多 3 次重试
func downloadCover(url, savePath string) error {
	// 从 URL 推断真实图片格式，避免固定 .jpg 后缀导致扩展名错误
	ext := ".jpg"
	if strings.HasSuffix(url, ".png") {
		ext = ".png"
	} else if strings.HasSuffix(url, ".webp") {
		ext = ".webp"
	} else if strings.HasSuffix(url, ".gif") {
		ext = ".gif"
	}
	actualSavePath := strings.TrimSuffix(savePath, filepath.Ext(savePath)) + ext

	// 使用带超时的独立 HTTP 客户端（不污染全局 DefaultClient）
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for retry := 0; retry < 3; retry++ {
		if retry > 0 {
			log.Printf("封面下载重试第 %d 次: %s", retry, url)
			time.Sleep(time.Duration(retry) * 2 * time.Second)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("创建封面请求失败: %w", err)
			continue
		}
		req.Header.Set("Referer", "https://www.bilibili.com")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("请求封面失败: %w", err)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("封面返回 HTTP %d", resp.StatusCode)
			continue
		}

		// 校验 Content-Type，确保响应确实是图片而非 HTML 错误页面
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && !strings.HasPrefix(contentType, "image/") {
			resp.Body.Close()
			lastErr = fmt.Errorf("封面响应 Content-Type 不是图片: %s", contentType)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取封面数据失败: %w", err)
			continue
		}

		if len(data) == 0 {
			lastErr = fmt.Errorf("封面数据为空")
			continue
		}

		if err := os.WriteFile(actualSavePath, data, 0644); err != nil {
			lastErr = fmt.Errorf("写入封面文件失败: %w", err)
			continue
		}

		log.Printf("封面下载成功: %s (%d bytes)", actualSavePath, len(data))
		return nil
	}

	return lastErr
}

// uniqueFilePath 如果目标路径已存在文件，在文件名后加 (N) 直到唯一
func uniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		newPath := fmt.Sprintf("%s(%d)%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}

// uniqueDirPath 如果目标目录已存在，在目录名后加 (N) 直到唯一
func uniqueDirPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	for i := 2; ; i++ {
		newPath := fmt.Sprintf("%s(%d)", path, i)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}

// writeCookieFile 将 B站 Cookie 写入 Netscape 格式的临时文件
// yt-dlp 的 --cookies 参数需要此格式
func (d *Downloader) writeCookieFile(cookie string) (string, error) {
	// 在程序专属临时目录下创建文件，避免残留到系统临时目录
	tmpFile, err := os.CreateTemp(d.tempDir, "bili-cookies-*.txt")
	if err != nil {
		return "", fmt.Errorf("创建临时 Cookie 文件失败: %w", err)
	}

	// 写入 Netscape cookie 文件格式
	// yt-dlp 要求的格式：domain\tpath\tsecure\texpiry\tname\tvalue
	content := "# Netscape HTTP Cookie File\n"
	content += ".bilibili.com\tTRUE\t/\tTRUE\t0\tCookie\t" + cookie + "\n"

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("写入 Cookie 文件失败: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// cleanupOldTempFiles 清理目录中的旧临时文件（上次程序异常退出残留）
func cleanupOldTempFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "bili-cookies-") {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

// sanitizedTitleContains 检查标题是否包含在文件名中
func sanitizedTitleContains(filename, title string) bool {
	// 移除常见的后缀如 (1), (2)
	re := regexp.MustCompile(`\(\d+\)$`)
	filename = re.ReplaceAllString(filename, "")

	return strings.Contains(filename, title) || strings.Contains(title, filename)
}

// sanitizeFilename 清理文件名，移除非法字符
// Windows 文件名不允许: \ / : * ? " < > |
// 同时处理路径穿越 ".."、null byte 和空文件名
func sanitizeFilename(filename string) string {
	// 替换 null byte
	result := strings.ReplaceAll(filename, "\x00", "")

	// 替换路径穿越
	result = strings.ReplaceAll(result, "..", "_")

	// 替换 Windows 非法字符
	invalidChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	// 限制文件名长度（按 rune 截断，避免切断 UTF-8 中文字符）
	runes := []rune(result)
	if len(runes) > 200 {
		runes = runes[:200]
	}
	result = string(runes)

	// 去除首尾空白
	result = strings.TrimSpace(result)

	// 如果清理后为空，使用默认文件名
	if result == "" {
		result = "unnamed"
	}

	return result
}

// GetVideoDuration 获取视频时长（通过 ffprobe）
// 如果未安装 ffprobe，返回 0
func GetVideoDuration(filePath string) (int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}

	var duration float64
	fmt.Sscanf(out.String(), "%f", &duration)

	return int(duration), nil
}

// FormatDuration 格式化时长为可读格式
// 例如: 3665 -> "1:01:05"
func FormatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
