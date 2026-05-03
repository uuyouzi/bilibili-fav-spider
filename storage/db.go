package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"bili-downloader/models"

	_ "modernc.org/sqlite"
)

// DB 数据库操作模块
// 负责所有与 SQLite 数据库的交互
// 使用 modernc.org/sqlite 驱动，纯 Go 实现，无需 C 编译器

const (
	// DatabaseFileName 数据库文件名
	DatabaseFileName = "videos.db"
)

// Storage 数据库存储结构体
type Storage struct {
	db *sql.DB    // SQLite 数据库连接
	mu sync.Mutex // 写操作互斥锁，防止并发写入导致 database is locked
}

// New 创建数据库连接并初始化表结构
// dbPath: 数据库文件路径
func New(dbPath string) (*Storage, error) {
	// 连接 SQLite 数据库
	// 如果数据库文件不存在，会自动创建
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库: %w", err)
	}

	// 设置连接池参数
	// MaxOpenConns=1 是 SQLite 的推荐设置，因为 SQLite 不支持并发写入
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// 验证数据库连接是否正常
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("无法连接数据库: %w", err)
	}

	s := &Storage{db: db}

	// 初始化表结构
	if err := s.initTables(); err != nil {
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return s, nil
}

// initTables 创建必要的数据库表
// 如果表已存在，则跳过（使用 IF NOT EXISTS）
func (s *Storage) initTables() error {
	// videos 表：存储视频信息
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS videos (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		bvid            TEXT    UNIQUE NOT NULL,           -- B站视频BV号，唯一标识
		title           TEXT    NOT NULL,                  -- 视频标题
		desc            TEXT    DEFAULT '',                -- 视频简介
		author          TEXT    DEFAULT '',                -- UP主名称
		author_mid      TEXT    DEFAULT '',                -- UP主mid
		duration        INTEGER DEFAULT 0,                 -- 视频时长（秒）
		pub_date        TEXT    DEFAULT '',                -- 发布时间
		favorite_time   TEXT    NOT NULL,                  -- 收藏时间（用于日期过滤）
		favorite_id     INTEGER DEFAULT 0,                 -- 收藏夹ID
		favorite_title  TEXT    DEFAULT '',                -- 收藏夹名称（用于目录分类）
		status          TEXT    NOT NULL DEFAULT 'pending', -- 下载状态
		save_path       TEXT    DEFAULT '',                -- 本地保存路径
		error_msg       TEXT    DEFAULT '',                -- 失败原因
		retries         INTEGER DEFAULT 0,                 -- 重试次数
		created_at      TEXT    NOT NULL,                  -- 记录创建时间
		updated_at      TEXT    NOT NULL                   -- 记录更新时间
	);

	-- 创建索引，加速按 bvid 和 status 查询
	CREATE INDEX IF NOT EXISTS idx_videos_bvid ON videos(bvid);
	CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
	CREATE INDEX IF NOT EXISTS idx_videos_favorite_id ON videos(favorite_id);
	`

	// 执行建表 SQL
	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	// 检查并添加新字段（兼容旧数据库）
	s.migrateAddFavoriteFields()

	return nil
}

// migrateAddFavoriteFields 迁移旧数据库，添加收藏夹字段
func (s *Storage) migrateAddFavoriteFields() {
	// 检查 favorite_id 字段是否存在
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('videos') WHERE name='favorite_id'").Scan(&count); err != nil {
		log.Printf("检查 favorite_id 字段失败: %v，跳过迁移", err)
		return
	}
	if count == 0 {
		if _, err := s.db.Exec("ALTER TABLE videos ADD COLUMN favorite_id INTEGER DEFAULT 0"); err != nil {
			log.Printf("添加 favorite_id 字段失败: %v", err)
		}
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('videos') WHERE name='favorite_title'").Scan(&count); err != nil {
		log.Printf("检查 favorite_title 字段失败: %v，跳过迁移", err)
		return
	}
	if count == 0 {
		if _, err := s.db.Exec("ALTER TABLE videos ADD COLUMN favorite_title TEXT DEFAULT ''"); err != nil {
			log.Printf("添加 favorite_title 字段失败: %v", err)
		}
	}
}

// Close 关闭数据库连接
func (s *Storage) Close() error {
	return s.db.Close()
}

// AddVideo 添加新视频到数据库
// 如果视频已存在（bvid 相同），则跳过，保留已有的下载状态和重试次数
func (s *Storage) AddVideo(video *models.Video) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")

	// 使用 INSERT OR IGNORE：
	// 如果 bvid 已存在（UNIQUE 约束），则忽略本次插入，保留已有记录
	// 避免覆盖已下载视频的状态（status、save_path、retries 等）
	sqlStr := `
	INSERT OR IGNORE INTO videos (
		bvid, title, desc, author, author_mid, duration,
		pub_date, favorite_time, favorite_id, favorite_title,
		status, save_path, error_msg, retries, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(sqlStr,
		video.Bvid,
		video.Title,
		video.Desc,
		video.Author,
		video.AuthorMid,
		video.Duration,
		video.PubDate.Format("2006-01-02 15:04:05"),
		video.FavoriteTime.Format("2006-01-02 15:04:05"),
		video.FavoriteId,
		video.FavoriteTitle,
		video.Status,
		video.SavePath,
		video.ErrorMsg,
		video.Retries,
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("添加视频失败: %w", err)
	}

	// 记录是否为新插入的记录
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("视频 %s 已存在，跳过插入", video.Bvid)
	}

	return nil
}

// GetVideoByBvid 根据 bvid 查询视频
func (s *Storage) GetVideoByBvid(bvid string) (*models.Video, error) {
	sqlStr := `
	SELECT id, bvid, title, desc, author, author_mid, duration,
	       pub_date, favorite_time, favorite_id, favorite_title,
	       status, save_path, error_msg, retries, created_at, updated_at
	FROM videos WHERE bvid = ?
	`

	row := s.db.QueryRow(sqlStr, bvid)

	video, err := scanVideo(row)
	if err != nil {
		// 如果是 sql.ErrNoRows，说明没有找到记录
		if err == sql.ErrNoRows {
			return nil, nil // 返回 nil, nil 表示视频不存在
		}
		return nil, fmt.Errorf("查询视频失败: %w", err)
	}

	return video, nil
}

// GetPendingVideos 获取所有待下载的视频
// 用于监控模块获取需要下载的视频列表
func (s *Storage) GetPendingVideos(limit int) ([]*models.Video, error) {
	sqlStr := `
	SELECT id, bvid, title, desc, author, author_mid, duration,
	       pub_date, favorite_time, favorite_id, favorite_title,
	       status, save_path, error_msg, retries, created_at, updated_at
	FROM videos
	WHERE status IN ('pending', 'failed')
	ORDER BY favorite_time ASC
	LIMIT ?
	`

	rows, err := s.db.Query(sqlStr, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待下载视频失败: %w", err)
	}
	defer rows.Close()

	return scanVideos(rows)
}

// UpdateVideoStatus 更新视频状态
func (s *Storage) UpdateVideoStatus(bvid string, status models.VideoStatus, savePath, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")

	sqlStr := `
	UPDATE videos
	SET status = ?, save_path = ?, error_msg = ?, updated_at = ?
	WHERE bvid = ?
	`

	_, err := s.db.Exec(sqlStr, status, savePath, errorMsg, now, bvid)
	if err != nil {
		return fmt.Errorf("更新视频状态失败: %w", err)
	}

	return nil
}

// IncrementRetries 增加视频的重试次数
func (s *Storage) IncrementRetries(bvid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sqlStr := `UPDATE videos SET retries = retries + 1 WHERE bvid = ?`
	_, err := s.db.Exec(sqlStr, bvid)
	return err
}

// GetStatistics 获取下载统计信息
// 返回：总数、已下载、待下载、失败、失效
func (s *Storage) GetStatistics() (map[string]int, error) {
	stats := make(map[string]int)

	// 查询总数
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM videos").Scan(&total); err != nil {
		return nil, err
	}
	stats["总数"] = total

	// 查询各状态数量
	statusList := []struct {
		status models.VideoStatus
		label  string
	}{
		{models.StatusDownloaded, "已下载"},
		{models.StatusPending, "待下载"},
		{models.StatusFailed, "下载失败"},
		{models.StatusExpired, "已失效"},
	}

	for _, item := range statusList {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM videos WHERE status = ?", item.status).Scan(&count); err != nil {
			return nil, err
		}
		stats[item.label] = count
	}

	return stats, nil
}

// scanVideo 将一行数据库记录扫描为 Video 结构体
func scanVideo(row *sql.Row) (*models.Video, error) {
	var v models.Video
	var pubDateStr, favoriteTimeStr, createdAtStr, updatedAtStr string

	err := row.Scan(
		&v.ID, &v.Bvid, &v.Title, &v.Desc, &v.Author, &v.AuthorMid, &v.Duration,
		&pubDateStr, &favoriteTimeStr, &v.FavoriteId, &v.FavoriteTitle,
		&v.Status, &v.SavePath, &v.ErrorMsg, &v.Retries, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	// 解析时间字符串为 time.Time
	var parseErr error
	v.PubDate, parseErr = time.Parse("2006-01-02 15:04:05", pubDateStr)
	if parseErr != nil {
		log.Printf("解析 pub_date 失败 (bvid=%s, value=%q): %v", v.Bvid, pubDateStr, parseErr)
	}
	v.FavoriteTime, parseErr = time.Parse("2006-01-02 15:04:05", favoriteTimeStr)
	if parseErr != nil {
		log.Printf("解析 favorite_time 失败 (bvid=%s, value=%q): %v", v.Bvid, favoriteTimeStr, parseErr)
	}
	v.CreatedAt, parseErr = time.Parse("2006-01-02 15:04:05", createdAtStr)
	if parseErr != nil {
		log.Printf("解析 created_at 失败 (bvid=%s, value=%q): %v", v.Bvid, createdAtStr, parseErr)
	}
	v.UpdatedAt, parseErr = time.Parse("2006-01-02 15:04:05", updatedAtStr)
	if parseErr != nil {
		log.Printf("解析 updated_at 失败 (bvid=%s, value=%q): %v", v.Bvid, updatedAtStr, parseErr)
	}

	return &v, nil
}

// scanVideos 将多行数据库记录扫描为 Video 切片
func scanVideos(rows *sql.Rows) ([]*models.Video, error) {
	var videos []*models.Video

	for rows.Next() {
		var v models.Video
		var pubDateStr, favoriteTimeStr, createdAtStr, updatedAtStr string

		err := rows.Scan(
			&v.ID, &v.Bvid, &v.Title, &v.Desc, &v.Author, &v.AuthorMid, &v.Duration,
			&pubDateStr, &favoriteTimeStr, &v.FavoriteId, &v.FavoriteTitle,
			&v.Status, &v.SavePath, &v.ErrorMsg, &v.Retries, &createdAtStr, &updatedAtStr,
		)
		if err != nil {
			return nil, err
		}

		// 解析时间字符串
		var parseErr error
		v.PubDate, parseErr = time.Parse("2006-01-02 15:04:05", pubDateStr)
		if parseErr != nil {
			log.Printf("解析 pub_date 失败 (bvid=%s, value=%q): %v", v.Bvid, pubDateStr, parseErr)
		}
		v.FavoriteTime, parseErr = time.Parse("2006-01-02 15:04:05", favoriteTimeStr)
		if parseErr != nil {
			log.Printf("解析 favorite_time 失败 (bvid=%s, value=%q): %v", v.Bvid, favoriteTimeStr, parseErr)
		}
		v.CreatedAt, parseErr = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if parseErr != nil {
			log.Printf("解析 created_at 失败 (bvid=%s, value=%q): %v", v.Bvid, createdAtStr, parseErr)
		}
		v.UpdatedAt, parseErr = time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if parseErr != nil {
			log.Printf("解析 updated_at 失败 (bvid=%s, value=%q): %v", v.Bvid, updatedAtStr, parseErr)
		}

		videos = append(videos, &v)
	}

	return videos, nil
}

// PrintStatistics 打印统计信息到控制台
func (s *Storage) PrintStatistics() {
	stats, err := s.GetStatistics()
	if err != nil {
		log.Printf("获取统计信息失败: %v", err)
		return
	}

	log.Println("\n========== 下载统计 ==========")
	log.Printf("总数:     %d", stats["总数"])
	log.Printf("已下载:   %d", stats["已下载"])
	log.Printf("待下载:   %d", stats["待下载"])
	log.Printf("下载失败: %d", stats["下载失败"])
	log.Printf("已失效:   %d", stats["已失效"])
	log.Println("==============================")
}

// LogVideoStates 打印所有视频的状态（用于调试）
func (s *Storage) LogVideoStates() {
	rows, err := s.db.Query("SELECT bvid, title, status FROM videos ORDER BY favorite_time DESC")
	if err != nil {
		log.Printf("查询视频状态失败: %v", err)
		return
	}
	defer rows.Close()

	log.Println("\n========== 视频状态列表 ==========")
	for rows.Next() {
		var bvid, title, status string
		rows.Scan(&bvid, &title, &status)
		log.Printf("[%s] %s - %s", status, bvid, title)
	}
	log.Println("==================================")
}

// MarkVideosAsIgnored 批量标记视频为"已隔离"
// 用于程序首次运行时，将 start_date 之前的视频标记为隔离状态
func (s *Storage) MarkVideosAsIgnored(startDate time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sqlStr := `
	UPDATE videos
	SET status = 'ignored', error_msg = '收藏时间早于起始日期，已隔离'
	WHERE favorite_time < ? AND status = 'pending'
	`

	result, err := s.db.Exec(sqlStr, startDate.Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// SearchVideos 搜索视频（按标题模糊搜索）
func (s *Storage) SearchVideos(keyword string) ([]*models.Video, error) {
	sqlStr := `
	SELECT id, bvid, title, desc, author, author_mid, duration,
	       pub_date, favorite_time, favorite_id, favorite_title,
	       status, save_path, error_msg, retries, created_at, updated_at
	FROM videos
	WHERE title LIKE ?
	ORDER BY favorite_time DESC
	`

	rows, err := s.db.Query(sqlStr, "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanVideos(rows)
}

// GetFailedVideos 获取所有下载失败的视频
func (s *Storage) GetFailedVideos() ([]*models.Video, error) {
	sqlStr := `
	SELECT id, bvid, title, desc, author, author_mid, duration,
	       pub_date, favorite_time, favorite_id, favorite_title,
	       status, save_path, error_msg, retries, created_at, updated_at
	FROM videos
	WHERE status = 'failed'
	ORDER BY favorite_time ASC
	`

	rows, err := s.db.Query(sqlStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanVideos(rows)
}

// BackupDatabase 备份数据库
// 将数据库复制到指定路径
func (s *Storage) BackupDatabase(backupPath string) error {
	// 读取原数据库文件
	data, err := os.ReadFile(DatabaseFileName)
	if err != nil {
		return fmt.Errorf("读取原数据库失败: %w", err)
	}

	// 写入备份文件
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	return nil
}
