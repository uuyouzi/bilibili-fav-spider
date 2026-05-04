package login

import (
	"encoding/base64"
	"fmt"
	"os"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/pkg/browser"
)

// DisplayQRCode 在浏览器中展示二维码
// loginURL: 需要编码进二维码的登录 URL
// 返回 HTML 文件路径（用于后续清理）
func DisplayQRCode(loginURL string) (string, error) {
	// 1. 生成二维码 PNG 图片
	imgPath, err := generateQRImage(loginURL)
	if err != nil {
		return "", fmt.Errorf("生成二维码图片失败: %w", err)
	}

	// 2. 将 PNG 转为 base64（嵌入 HTML，不依赖外部文件）
	pngData, err := os.ReadFile(imgPath)
	if err != nil {
		os.Remove(imgPath)
		return "", fmt.Errorf("读取二维码图片失败: %w", err)
	}
	os.Remove(imgPath) // 图片已读入内存，删除临时文件

	base64img := base64.StdEncoding.EncodeToString(pngData)

	// 3. 构造 HTML 页面
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>B站扫码登录 - bili-downloader</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
    display: flex; justify-content: center; align-items: center;
    min-height: 100vh; background: #f5f5f5;
    font-family: -apple-system, "Microsoft YaHei", sans-serif;
}
.card {
    background: #fff; border-radius: 12px; padding: 40px;
    box-shadow: 0 4px 20px rgba(0,0,0,0.1); text-align: center;
    max-width: 360px;
}
h2 { color: #333; margin-bottom: 8px; }
p { color: #999; font-size: 14px; margin-bottom: 24px; }
.qr-wrap {
    border: 2px solid #fb7299; border-radius: 8px;
    padding: 12px; display: inline-block;
}
.qr-wrap img { width: 200px; height: 200px; display: block; }
.steps { text-align: left; margin-top: 24px; color: #666; font-size: 14px; line-height: 2; }
.steps span { color: #fb7299; font-weight: bold; margin-right: 8px; }
.footer { margin-top: 16px; color: #bbb; font-size: 12px; }
</style>
</head>
<body>
<div class="card">
    <h2>B站扫码登录</h2>
    <p>请使用 Bilibili App 扫描二维码</p>
    <div class="qr-wrap">
        <img src="data:image/png;base64,%s" alt="QR Code">
    </div>
    <div class="steps">
        <div><span>1</span>打开 Bilibili App</div>
        <div><span>2</span>点击右上角扫码图标</div>
        <div><span>3</span>扫描上方二维码</div>
        <div><span>4</span>在手机上确认登录</div>
    </div>
    <div class="footer">bili-downloader · 扫码完成后可关闭此页面</div>
</div>
</body>
</html>`, base64img)

	// 4. 写入临时文件并用浏览器打开
	tmpFile, err := os.CreateTemp("", "bili-login-*.html")
	if err != nil {
		return "", fmt.Errorf("创建临时HTML文件失败: %w", err)
	}
	htmlPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		os.Remove(htmlPath)
		return "", fmt.Errorf("写入HTML文件失败: %w", err)
	}
	tmpFile.Close()

	if err := browser.OpenFile(htmlPath); err != nil {
		os.Remove(htmlPath)
		return "", fmt.Errorf("打开浏览器失败: %w", err)
	}

	return htmlPath, nil
}

// generateQRImage 生成二维码 PNG 图片
func generateQRImage(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "bili-qr-*.png")
	if err != nil {
		return "", fmt.Errorf("创建临时PNG文件失败: %w", err)
	}
	imgPath := tmpFile.Name()
	tmpFile.Close()

	if err := qrcode.WriteFile(content, qrcode.Medium, 256, imgPath); err != nil {
		os.Remove(imgPath)
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}

	return imgPath, nil
}

// cleanupFile 删除临时文件（忽略错误）
func cleanupFile(path string) {
	os.Remove(path)
}
