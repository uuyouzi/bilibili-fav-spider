package monitor

import (
	"fmt"
	"log"
	"sync"
	"time"

	"bili-downloader/api"
	"bili-downloader/downloader"
	"bili-downloader/models"
	"bili-downloader/storage"
)

// Monitor 收藏夹监控模块
// 负责定时检查收藏夹，发现新视频并下载

// Monitor 监控器结构体
type Monitor struct {
	// 配置
	config *models.Config

	// B站 API 客户端
	biliAPI *api.BilibiliClient

	// 下载器
	downloader *downloader.Downloader

	// 数据库
	storage *storage.Storage

	// 起始日期（只处理此日期之后的收藏）
	startDate time.Time

	// 运行状态
	running bool
	mu      sync.Mutex // 保护 running 状态

	// 停止信号通道
	stopChan chan struct{}

	// 统计信息
	stats *Stats
}

// Stats 统计信息
type Stats struct {
	TotalChecked int // 总共检查的视频数
	NewVideos    int // 发现的新视频数
	Downloaded   int // 成功下载数
	Failed       int // 下载失败数
	Skipped      int // 跳过的视频数（已存在/已失效/日期隔离）
	mu           sync.Mutex
}

// New 创建监控器实例
func New(cfg *models.Config, biliAPI *api.BilibiliClient, dl *downloader.Downloader, store *storage.Storage) (*Monitor, error) {
	// 解析起始日期
	startDate, err := time.Parse("2006-01-02", cfg.StartDate)
	if err != nil {
		return nil, err
	}

	return &Monitor{
		config:     cfg,
		biliAPI:    biliAPI,
		downloader: dl,
		storage:    store,
		startDate:  startDate,
		running:    false,
		stopChan:   make(chan struct{}),
		stats:      &Stats{},
	}, nil
}

// Start 启动监控
// 这是一个阻塞方法，会持续运行直到调用 Stop()
func (m *Monitor) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	log.Println("========================================")
	log.Println("B站收藏夹自动下载器已启动")
	log.Printf("起始日期: %s（只监控此日期后的收藏）", m.config.StartDate)
	log.Printf("检查间隔: %d 分钟", m.config.CheckIntervalMinutes)
	log.Printf("保存路径: %s", m.config.SavePath)
	log.Printf("下载质量: %s", m.config.DownloadQuality)
	log.Println("========================================")
	log.Println("按 Ctrl+C 停止程序")
	log.Println()

	// 首次运行：全量同步
	log.Println(">>> 首次运行：正在同步收藏夹...")
	if err := m.syncFavorites(); err != nil {
		log.Printf("首次同步失败: %v", err)
	}

	// 定时检查
	ticker := time.NewTicker(time.Duration(m.config.CheckIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// 定期打印统计信息
	statsTicker := time.NewTicker(1 * time.Hour)
	defer statsTicker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定时检查收藏夹
			log.Println("\n>>> 定时检查：正在检查收藏夹...")
			if err := m.syncFavorites(); err != nil {
				log.Printf("检查失败: %v", err)
			}

		case <-statsTicker.C:
			// 定期打印统计
			m.printStats()

		case <-m.stopChan:
			// 收到停止信号
			log.Println("收到停止信号，正在关闭...")
			return nil
		}
	}
}

// Stop 停止监控
// 关闭 stopChan，通知 Start() 方法退出
func (m *Monitor) Stop() {
	m.mu.Lock()
	if m.running {
		m.running = false
		close(m.stopChan) // 通知 Start() 退出循环
	}
	m.mu.Unlock()
}

// syncFavorites 同步收藏夹
// 从 B站 获取收藏夹列表，与本地数据库对比，找出新增/需要下载的视频
func (m *Monitor) syncFavorites() error {
	// 获取所有收藏夹的视频
	log.Println("正在从B站获取收藏夹信息...")
	videos, err := m.biliAPI.GetAllFavoritesVideos(m.config.MonitoredFavorites)
	if err != nil {
		return err
	}

	log.Printf("获取到 %d 个收藏视频", len(videos))

	// 用于记录统计
	newCount := 0
	skipCount := 0

	// 遍历每个视频
	for _, video := range videos {
		// 1. 检查是否在日期范围内
		if video.FavoriteTime.Before(m.startDate) {
			// 收藏时间早于起始日期，跳过
			skipCount++
			log.Printf("跳过（日期隔离）: %s - %s", video.Bvid, video.Title)
			continue
		}

		// 2. 检查是否已存在
		existing, err := m.storage.GetVideoByBvid(video.Bvid)
		if err != nil {
			log.Printf("查询数据库失败: %v", err)
			continue
		}

		if existing != nil {
			// 视频已存在，检查状态
			skipCount++
			continue
		}

		// 3. 新视频，添加到数据库
		if err := m.storage.AddVideo(video); err != nil {
			log.Printf("添加视频到数据库失败: %v", err)
			continue
		}

		newCount++
		log.Printf("发现新视频: %s - %s (收藏于 %s)",
			video.Bvid, video.Title, video.FavoriteTime.Format("2006-01-02"))
	}

	log.Printf("\n同步完成！新增: %d, 跳过: %d", newCount, skipCount)

	// 如果有新增视频，开始下载
	if newCount > 0 {
		log.Println("\n>>> 开始下载新增视频...")
		m.downloadPending()
	}

	// 打印统计
	m.printStats()

	return nil
}

// downloadPending 下载待下载的视频
func (m *Monitor) downloadPending() {
	// 获取待下载的视频
	videos, err := m.storage.GetPendingVideos(0) // 0 = 不限制数量
	if err != nil {
		log.Printf("获取待下载视频失败: %v", err)
		return
	}

	if len(videos) == 0 {
		log.Println("没有待下载的视频")
		return
	}

	log.Printf("发现 %d 个待下载视频", len(videos))

	// 限制并发数
	maxConcurrent := m.config.MaxConcurrentDownloads
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	// 使用信号量控制并发
	sem := make(chan struct{}, maxConcurrent)
	results := make(chan struct {
		video *models.Video
		ok    bool
		err   error
	}, len(videos))

	// 启动下载任务
	for i, video := range videos {
		go func(idx int, v *models.Video) {
			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 防止 goroutine panic 导致 results channel 永远收不到结果而死锁
			defer func() {
				if r := recover(); r != nil {
					log.Printf("下载 goroutine 异常 (bvid=%s): %v", v.Bvid, r)
					results <- struct {
						video *models.Video
						ok    bool
						err   error
					}{video: v, ok: false, err: fmt.Errorf("panic: %v", r)}
				}
			}()

			log.Printf("[%d/%d] 准备下载: %s", idx+1, len(videos), v.Title)

			// 检查是否可下载
			if v.Status == models.StatusExpired {
				log.Printf("视频已失效，跳过: %s", v.Title)
				results <- struct {
					video *models.Video
					ok    bool
					err   error
				}{video: v, ok: false, err: nil}
				return
			}

			// 根据下载模式选择处理方式
			var result *downloader.DownloadResult
			if m.config.DownloadMode == "metadata" {
				result = m.downloader.SaveMetadata(v, m.config.IsFavoriteFolders())
			} else {
				result = m.downloader.Download(v, m.config.Cookie, m.config.IsFavoriteFolders())
			}
			if result.Success {
				// 更新数据库
				m.storage.UpdateVideoStatus(v.Bvid, models.StatusDownloaded, result.SavePath, "")
				results <- struct {
					video *models.Video
					ok    bool
					err   error
				}{video: v, ok: true, err: nil}
			} else {
				// 更新数据库为失败
				m.storage.UpdateVideoStatus(v.Bvid, models.StatusFailed, "", result.ErrorMsg)
				m.storage.IncrementRetries(v.Bvid)
				results <- struct {
					video *models.Video
					ok    bool
					err   error
				}{video: v, ok: false, err: nil}
			}
		}(i, video)
	}

	// 等待所有下载完成
	successCount := 0
	failCount := 0
	expiredCount := 0

	for i := 0; i < len(videos); i++ {
		result := <-results
		if result.ok {
			successCount++
			m.stats.mu.Lock()
			m.stats.Downloaded++
			m.stats.mu.Unlock()
		} else if result.video != nil && result.video.Status == models.StatusExpired {
			expiredCount++
		} else {
			failCount++
			m.stats.mu.Lock()
			m.stats.Failed++
			m.stats.mu.Unlock()
		}
	}

	log.Printf("\n本次下载完成! 成功: %d, 失败: %d, 失效: %d",
		successCount, failCount, expiredCount)
}

// printStats 打印统计信息
func (m *Monitor) printStats() {
	m.storage.PrintStatistics()
}

// SyncOnce 执行一次同步（用于手动触发）
func (m *Monitor) SyncOnce() error {
	return m.syncFavorites()
}

// RetryFailed 重试下载失败的视频
func (m *Monitor) RetryFailed() {
	failedVideos, err := m.storage.GetFailedVideos()
	if err != nil {
		log.Printf("获取失败视频失败: %v", err)
		return
	}

	if len(failedVideos) == 0 {
		log.Println("没有需要重试的失败视频")
		return
	}

	log.Printf("准备重试 %d 个失败视频...", len(failedVideos))

	for _, video := range failedVideos {
		log.Printf("重试下载: %s", video.Title)
		result := m.downloader.Download(video, m.config.Cookie, m.config.IsFavoriteFolders())

		if result.Success {
			m.storage.UpdateVideoStatus(video.Bvid, models.StatusDownloaded, result.SavePath, "")
			log.Printf("重试成功: %s", video.Title)
		} else {
			log.Printf("重试失败: %s - %s", video.Title, result.ErrorMsg)
		}
	}
}
