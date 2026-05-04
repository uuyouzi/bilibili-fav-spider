package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"bili-downloader/api"
	"bili-downloader/config"
	"bili-downloader/downloader"
	"bili-downloader/login"
	"bili-downloader/monitor"
	"bili-downloader/storage"
)

// 程序版本
const Version = "1.0.0"

func main() {
	// 打印欢迎信息
	printBanner()

	// 设置日志文件（同时输出到控制台和文件）
	logFile, err := os.OpenFile("bili-downloader.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		defer logFile.Close()
	}

	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径（默认为当前目录下的 config.yaml）")
	singleRun := flag.Bool("once", false, "仅执行一次同步，不持续监控")
	showStats := flag.Bool("stats", false, "仅显示统计信息并退出")
	flag.Parse()

	// 1. 加载配置
	log.Println("正在加载配置...")
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	log.Println("配置加载成功!")

	// 2. 确保保存目录存在
	if err := config.EnsureSavePathExists(cfg.SavePath); err != nil {
		log.Fatalf("无法创建保存目录: %v", err)
	}

	// 3. 创建数据库
	log.Println("正在初始化数据库...")
	store, err := storage.New(storage.DatabaseFileName)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer store.Close()
	log.Println("数据库初始化成功!")

	// 4. 如果未配置 Cookie 且未配置 UID，启动扫码登录
	if cfg.Cookie == "" && cfg.UID == "" {
		result, err := login.StartQRLogin()
		if err != nil {
			log.Fatalf("扫码登录失败: %v", err)
		}
		cfg.Cookie = result.Cookie
		log.Println("扫码登录成功！")
	}

	// 5. 创建 B站 API 客户端
	biliAPI := api.NewBilibiliClient(cfg.Cookie, cfg.UID)

	// 6. 验证 Cookie 是否有效（UID 模式下跳过）
	if cfg.UID != "" {
		log.Println("UID 模式，跳过 Cookie 验证")
	} else {
		log.Println("正在验证 Cookie...")
		if err := biliAPI.ValidateCookie(); err != nil {
			log.Fatalf("Cookie 验证失败: %v", err)
		}
	}

	// 7. 创建下载器（UID 模式下按 UID 分目录）
	savePath := cfg.SavePath
	if cfg.UID != "" {
		savePath = filepath.Join(savePath, cfg.UID)
		log.Printf("UID 模式，保存到: %s", savePath)
	}
	dlConfig := &downloader.Config{
		SavePath:            savePath,
		Quality:             cfg.DownloadQuality,
		Timeout:             cfg.DownloadTimeout,
		MaxRetries:          3,
		EnableNotification:  cfg.EnableNotification,
	}
	dl := downloader.New(dlConfig)

	// 7. 检查 yt-dlp 是否安装（metadata 模式下不需要）
	if cfg.DownloadMode != "metadata" {
		log.Println("正在检查 yt-dlp...")
		if err := dl.CheckYtDlp(); err != nil {
			log.Fatalf("yt-dlp 检查失败: %v", err)
		}
	} else {
		log.Println("当前为元数据模式，跳过 yt-dlp 检查")
	}

	// 8. 处理命令行选项
	if *showStats {
		// 仅显示统计信息
		store.PrintStatistics()
		return
	}

	if *singleRun {
		// 仅执行一次同步
		m, err := monitor.New(cfg, biliAPI, dl, store)
		if err != nil {
			log.Fatalf("创建监控器失败: %v", err)
		}
		if err := m.SyncOnce(); err != nil {
			log.Fatalf("同步失败: %v", err)
		}
		return
	}

	// 9. 创建并启动监控器
	m, err := monitor.New(cfg, biliAPI, dl, store)
	if err != nil {
		log.Fatalf("创建监控器失败: %v", err)
	}

	// 10. 设置信号处理
	// 当用户按 Ctrl+C 时，优雅地关闭程序
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在后台运行监控器
	go func() {
		if err := m.Start(); err != nil {
			log.Printf("监控器异常退出: %v", err)
		}
	}()

	// 11. 等待信号
	<-sigChan
	log.Println("\n正在停止监控器...")

	// 打印最终统计
	store.PrintStatistics()

	log.Println("程序已退出，感谢使用!")
}

// printBanner 打印程序横幅
func printBanner() {
	fmt.Print(`
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║         B站收藏夹自动下载器  v` + Version + `                      ║
║                                                           ║
║  功能: 自动监控B站收藏夹，新收藏视频自动下载到本地      ║
║  特性: 多收藏夹支持 | 日期过滤 | 去重 | 最佳清晰度      ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
`)
}
