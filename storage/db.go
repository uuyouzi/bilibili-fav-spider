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

const (
	DatabaseFileName = "videos.db"
)

type Storage struct {
	db *sql.DB
	mu sync.Mutex
}

func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("无法连接数据库: %w", err)
	}

	s := &Storage{db: db}

	if err := s.initTables(); err != nil {
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return s, nil
}

func (s *Storage) initTables() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS videos (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		bvid            TEXT    UNIQUE NOT NULL,
		title           TEXT    NOT NULL,
		desc            TEXT    DEFAULT '',
		author          TEXT    DEFAULT '',
		author_mid      TEXT    DEFAULT '',
		duration        INTEGER DEFAULT 0,
		pub_date        TEXT    DEFAULT '',
		favorite_time   TEXT    NOT NULL,
		favorite_id     INTEGER DEFAULT 0,
		favorite_title  TEXT    DEFAULT '',
		status          TEXT    NOT NULL DEFAULT 'pending',
		save_path       TEXT    DEFAULT '',
		error_msg       TEXT    DEFAULT '',
		retries         INTEGER DEFAULT 0,
		created_at      TEXT    NOT NULL,
		updated_at      TEXT    NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_videos_bvid ON videos(bvid);
	CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
	CREATE INDEX IF NOT EXISTS idx_videos_favorite_id ON videos(favorite_id);
	`

	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	s.migrateAddFavoriteFields()

	return nil
}

func (s *Storage) migrateAddFavoriteFields() {
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

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) AddVideo(video *models.Video) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")

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

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("视频 %s 已存在，跳过插入", video.Bvid)
	}

	return nil
}

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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询视频失败: %w", err)
	}

	return video, nil
}

// GetPendingVideos 获取待下载的视频
// limit: 最大数量，0 表示不限制
func (s *Storage) GetPendingVideos(limit int) ([]*models.Video, error) {
	sqlStr := `
	SELECT id, bvid, title, desc, author, author_mid, duration,
	       pub_date, favorite_time, favorite_id, favorite_title,
	       status, save_path, error_msg, retries, created_at, updated_at
	FROM videos
	WHERE status IN ('pending', 'failed')
	ORDER BY favorite_time ASC`

	args := []interface{}{}
	if limit > 0 {
		sqlStr += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("查询待下载视频失败: %w", err)
	}
	defer rows.Close()

	return scanVideos(rows)
}

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

func (s *Storage) IncrementRetries(bvid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sqlStr := `UPDATE videos SET retries = retries + 1 WHERE bvid = ?`
	_, err := s.db.Exec(sqlStr, bvid)
	return err
}

func (s *Storage) GetStatistics() (map[string]int, error) {
	stats := make(map[string]int)

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM videos").Scan(&total); err != nil {
		return nil, err
	}
	stats["总数"] = total

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

func (s *Storage) BackupDatabase(backupPath string) error {
	data, err := os.ReadFile(DatabaseFileName)
	if err != nil {
		return fmt.Errorf("读取原数据库失败: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	return nil
}
