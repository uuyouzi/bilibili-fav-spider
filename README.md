# B站收藏夹自动下载器

> 自动监控 B站收藏夹，新收藏视频自动下载到本地硬盘，再也不怕视频被删了！

## 功能特性

- ✅ **自动监控**：定时检查收藏夹，新收藏的视频自动下载
- ✅ **扫码登录**：无需手动复制 Cookie，浏览器扫码即可登录
- ✅ **多收藏夹支持**：支持同时监控多个收藏夹，比如AI学习收藏夹、音乐收藏夹等
- ✅ **按收藏夹分类**：视频自动按收藏夹名称分目录保存
- ✅ **元数据备份**：可仅保存标题和描述为 txt 文件，不怕视频下架
- ✅ **日期过滤**：只监控指定日期后的收藏，之前的自动隔离
- ✅ **智能去重**：已下载的视频不会重复下载
- ✅ **最佳画质**：自动下载你能获取的最高画质（无大会员最高 1080P）
- ✅ **失效检测**：自动识别已被下架的视频
- ✅ **断点续传**：程序中断后重启会继续下载
- ✅ **纯 Go 实现**：编译后是单个可执行文件，无需安装运行环境

## 目录

- [快速开始](#快速开始)
- [详细安装教程](#详细安装教程)
- [使用说明](#使用说明)
- [配置说明](#配置说明)
- [常见问题](#常见问题)
- [项目结构](#项目结构)
- [命令行参数](#命令行参数)

---

## 快速开始

### 第一步：安装前置工具

本程序依赖 `yt-dlp` 来下载视频，这是目前最强的 B站 视频下载工具。

**Windows 用户：**

打开 PowerShell（管理员），运行：

```powershell
# 方法1：使用 winget（推荐）
winget install yt-dlp.yt-dlp

# 方法2：使用 pip
pip install yt-dlp

# 方法3：手动下载
# 访问 https://github.com/yt-dlp/yt-dlp/releases
# 下载最新的 yt-dlp.exe 文件
# 将其放到系统 PATH 或程序同目录下
```

**macOS 用户：**

```bash
brew install yt-dlp
```

**Linux 用户：**

```bash
# Ubuntu/Debian
sudo apt install yt-dlp

# 或使用 pip
pip install yt-dlp
```

安装完成后，在终端验证：

```bash
yt-dlp --version
# 应该显示类似：2024.01.01
```

### 第二步：安装本程序

**方式一：编译运行（推荐给开发者）**

```bash
# 1. 确保已安装 Go 1.21 或更高版本
go version

# 2. 克隆或下载本项目到本地

# 3. 进入项目源码目录
cd bili-downloader/src

# 4. 下载依赖
go mod tidy

# 5. 编译程序
go build -o ../bin/bili-downloader.exe

# 6. 返回根目录运行
cd .. && ./bin/bili-downloader.exe
```

**方式二：直接下载预编译版本**

> 等待后续 release，届时会提供 Windows/macOS/Linux 的可执行文件下载。

### 第三步：登录 B站

**推荐方式：扫码登录** ✨

把 `config.yaml` 中的 `cookie` 留空即可，启动程序时会自动打开浏览器展示二维码，用手机 B站 App 扫码确认即完成登录。

**备用方式：手动获取 Cookie**

如果需要手动配置 Cookie：

1. 打开 Chrome 浏览器，访问 [bilibili.com](https://bilibili.com)，**登录你的账号**
2. 按 `F12` 打开开发者工具
3. 点击顶部的 **Network（网络）** 标签
4. 按 `F5` 刷新页面
5. 在左侧列表中点击任意一个请求（如 `nav` 或 `main`）
6. 在右侧找到 **Request Headers（请求头）**
7. 找到 `Cookie` 字段，复制整个值（很长的一串字符）

> 示意图：
> ```
> Cookie: SESSDATA=xxxxxxxxxx; BILI_JWT=xxxxxx; ...
> ```

### 第四步：编辑配置文件

将 `config.yaml.example` 重命名为 `config.yaml`（去掉 `.example`），然后编辑：

```yaml
# 登录方式（二选一）
# 方式1：留空，启动时扫码登录（推荐）
cookie: ""
# 方式2：手动填写 Cookie
# cookie: "你的Cookie值"

# 视频保存路径（必填）
save_path: "D:/B站视频"

# 起始日期（必填）
# 只监控此日期之后收藏的视频
start_date: "2026-04-01"

# 其他配置可以使用默认值
check_interval_minutes: 30
max_concurrent_downloads: 1
```

### 第五步：运行程序

```bash
# 正常运行（持续监控）
./bin/bili-downloader.exe

# 或仅执行一次同步
./bin/bili-downloader.exe --once

# 或仅查看统计
./bin/bili-downloader.exe --stats
```

---

## 详细安装教程

### 一、什么是 Cookie？为什么需要它？

Cookie 是你在 B站 登录后服务器给你的"身份证明"，类似于临时身份证。

- **有了 Cookie**，程序才能以你的身份访问你的私人收藏夹
- **没有 Cookie**，程序只能看到公开信息，无法访问你的收藏夹
- **Cookie 有有效期**，一般 180 天左右，过期后需要重新获取

### 二、获取 Cookie 的详细步骤

#### 方法一：从浏览器开发者工具获取（推荐）

1. **打开 Chrome**，访问 [bilibili.com](https://bilibili.com)
2. **确保已登录**（右上角显示你的头像和用户名）
3. 按 `F12` 打开开发者工具
4. 点击顶部菜单栏的 **Network**（网络）标签
5. 按 `F5` 刷新页面
6. 在左侧请求列表中，**双击**任意一个请求（如 `nav`）
7. 在右侧面板的 **Headers** > **Request Headers** 中找到 `Cookie`
8. **复制整个 Cookie 值**（从 `SESSDATA=` 开始到结尾的所有内容）

#### 方法二：从浏览器扩展获取

如果你安装了 EditThisCookie 等扩展，可以直接点击扩展图标导出 Cookie。

#### 方法三：从浏览器存储中提取

1. 在 B站 页面按 `F12`
2. 切换到 **Application（应用）** 标签
3. 左侧找到 **Cookies** > `https://www.bilibili.com`
4. 找到 `SESSDATA` 行，复制 `Value` 列的值

#### 方法四：扫码登录（推荐✨）

只需在 `config.yaml` 中把 `cookie` 留空或删除：

```yaml
cookie: ""
```

启动程序时会**自动打开浏览器显示二维码**，用手机 B站 App 扫码即可登录，无需手动复制 Cookie。

> **优点**：操作简单，不怕 Cookie 过期（每次启动重新扫一次即可）
> 需要浏览器支持，本地启动时会自动调用默认浏览器

### 三、安装 Go 开发环境（如果你想编译程序）

**Windows：**

1. 访问 https://go.dev/dl/
2. 下载 Windows 安装包（如 `go1.21.windows-amd64.msi`）
3. 运行安装包，一路下一步
4. 打开 PowerShell，运行 `go version` 验证

**macOS：**

```bash
brew install go
```

### 四、常见安装错误及解决

#### 错误1：`yt-dlp: command not found`

**原因**：yt-dlp 没有安装或不在系统 PATH 中

**解决**：
```powershell
# 使用完整路径运行
C:/Users/你的用户名/AppData/Local/Programs/yt-dlp/yt-dlp.exe --version

# 或者把 yt-dlp.exe 复制到程序同目录下
```

#### 错误2：`go: command not found`

**原因**：Go 没有安装或不在系统 PATH 中

**解决**：重新安装 Go，确保勾选 "Add to PATH"

#### 错误3：`SESSDATA expired`

**原因**：Cookie 过期了

**解决**：重新获取 Cookie（按照上面的步骤）

---

## 使用说明

### 运行模式

本程序有三种运行模式：

#### 1. 持续监控模式（默认）

```bash
./bin/bili-downloader.exe
```

程序会：
- 首次运行时同步所有收藏夹
- 每隔配置的间隔时间自动检查
- 发现新视频立即下载
- 持续运行直到你按 `Ctrl+C` 停止

#### 2. 单次同步模式

```bash
./bin/bili-downloader.exe --once
```

程序会：
- 执行一次同步检查
- 下载所有待下载的视频
- 然后自动退出

适合场景：配合 Windows 任务计划程序定时执行。

#### 3. 仅查看统计

```bash
./bin/bili-downloader.exe --stats
```

程序会：
- 读取本地数据库
- 显示所有视频的下载统计
- 然后退出

### 停止程序

- 按 `Ctrl+C` 可以优雅地停止程序
- 程序会打印最终的下载统计
- 下次启动会从上次中断的地方继续

---

## 配置说明

### 完整配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `cookie` | 字符串 | 可选 | B站登录 Cookie（留空则启动时扫码登录） |
| `save_path` | 字符串 | **必填** | 视频保存根目录 |
| `start_date` | 字符串 | **必填** | 起始日期，格式 `YYYY-MM-DD` |
| `check_interval_minutes` | 整数 | `30` | 检查间隔（分钟） |
| `max_concurrent_downloads` | 整数 | `1` | 最大并发下载数 |
| `download_quality` | 字符串 | `bestvideo+bestaudio` | 下载质量 |
| `download_timeout_seconds` | 整数 | `3600` | 单个视频超时（秒） |
| `download_mode` | 字符串 | `video` | 下载模式：`video` 下载视频 / `metadata` 仅保存标题描述 |
| `enable_notification` | 布尔 | `true` | 是否打印详细信息 |
| `favorite_folders` | 布尔 | `true` | 是否按收藏夹分类目录 |
| `monitored_favorites` | 列表 | `[]` | 要监控的收藏夹名称列表 |

### 多收藏夹配置详解

#### 1. `favorite_folders` - 目录分类方式

```yaml
# true = 按收藏夹分类（推荐）
# 保存路径: {save_path}/{收藏夹名称}/{UP主}/{视频标题}
favorite_folders: true

# false = 不按收藏夹分类
# 保存路径: {save_path}/{UP主}/{视频标题}
favorite_folders: false
```

#### 2. `monitored_favorites` - 指定要监控的收藏夹

```yaml
# 监控所有收藏夹
monitored_favorites: []

# 只监控特定收藏夹（支持模糊匹配）
monitored_favorites:
  - "默认收藏夹"
  - "AI"
  - "音乐"
```

**示例场景：**
- 你有"AI"、"默认收藏夹"、"音乐"三个收藏夹
- 设置 `monitored_favorites: ["AI", "音乐"]` 只会监控这两个
- "默认收藏夹"完全不会被检查

#### 3. 实际目录结构示例

假设你配置如下：
```yaml
save_path: "D:/B站视频"
favorite_folders: true
monitored_favorites: ["AI", "音乐"]
```

程序运行后，会生成这样的目录结构：
```
D:/B站视频/
├── AI/
│   ├── 李子柒/
│   │   └── 田园生活记录.mp4
│   └── 华农兄弟/
│       └── 竹鼠养殖指南.mp4
├── 音乐/
│   ├── 周杰伦/
│   │   └── 晴天.mp4
│   └── 林俊杰/
│       └──江南.mp4
└── 默认收藏夹/    ← 这个收藏夹不会被监控（未在列表中）
    └── ...
```

### 配置示例

#### 场景1：日常使用（默认配置）

```yaml
cookie: "SESSDATA=xxxxx"
save_path: "D:/B站视频"
start_date: "2026-04-01"
favorite_folders: true           # 按收藏夹分类
monitored_favorites: []         # 监控所有收藏夹
```

#### 场景2：只监控特定收藏夹

```yaml
cookie: "SESSDATA=xxxxx"
save_path: "D:/B站视频"
start_date: "2026-04-01"
favorite_folders: true           # 按收藏夹分类
monitored_favorites:            # 只监控以下收藏夹
  - "AI"
  - "音乐"
```

#### 场景3：密集下载（提高并发）

```yaml
cookie: "SESSDATA=xxxxx"
save_path: "D:/B站视频"
start_date: "2026-04-01"
favorite_folders: true
monitored_favorites: []
check_interval_minutes: 60      # 间隔1小时
max_concurrent_downloads: 2      # 同时下载2个
download_timeout_seconds: 7200   # 超时2小时
```

#### 场景4：低带宽环境

```yaml
cookie: "SESSDATA=xxxxx"
save_path: "D:/B站视频"
start_date: "2026-04-01"
favorite_folders: false          # 不按收藏夹分类
monitored_favorites: []
check_interval_minutes: 120     # 间隔2小时
max_concurrent_downloads: 1     # 只下载1个
download_timeout_seconds: 10800 # 超时3小时
```

### 关于 `start_date` 的说明

`start_date` 是一个非常实用的功能，可以帮你：

- **新项目启动**：只监控之后的收藏，之前的视频单独处理
- **时间节点筛选**：比如"只保存今年的收藏"
- **分期备份**：可以分批设置不同的日期进行备份

### 关于 `download_quality` 的说明

| 值 | 说明 | 画质 | 文件格式 |
|----|------|------|----------|
| `bestvideo+bestaudio` | 分离下载再合并 | 最高 | mkv/mp4 |
| `best` | 单文件下载 | 次高 | flv/mp4 |

> 推荐使用 `bestvideo+bestaudio`，这是最高画质选项。

### 关于 `download_mode` 的说明

| 值 | 说明 | 需要 yt-dlp | 输出 |
|----|------|-------------|------|
| `video` | 下载完整视频（默认） | 是 | `.mp4`/`.mkv` 视频文件 |
| `metadata` | 仅保存标题和描述 | **否** | `.txt` 文本文件 |

**metadata 模式适用场景：**
- 只想备份收藏夹中每个视频的标题、描述等信息，不需要视频本身
- 担心视频被下架后，连标题都看不到，不知道丢的是什么内容
- 没有安装 yt-dlp 或磁盘空间不够，只想先记录有哪些视频

**metadata 模式保存的文件示例：**
```
标题: 田园生活记录
BV号: BV1xx4y1d7z9
UP主: 李子柒 (mid: 123456)
收藏夹: AI
收藏时间: 2026-04-01 12:00:00
发布时间: 2026-03-15 10:00:00
时长: 5:30
链接: https://www.bilibili.com/video/BV1xx4y1d7z9
描述:
这是一个关于田园生活的视频...
```

---

## 常见问题

### Q1: 程序报 "Cookie 无效" 错误

**A**: 
- 最简单的方法：删除 config.yaml 中的 cookie 值（设为空），重启程序使用扫码登录
- 或者手动重新获取：参考上文"获取 Cookie"的步骤

### Q2: 程序报 "未找到 yt-dlp" 错误

**A**: yt-dlp 没有安装或不在 PATH 中。请参考上文"安装前置工具"的步骤安装。

### Q3: 下载的视频画质不够清晰

**A**: 
1. 检查你的账号是否有大会员，没有会员最高只能下载 1080P
2. 确认 `download_quality` 设置为 `bestvideo+bestaudio`

### Q4: 视频下载到一半就失败了

**A**: 
1. 网络不稳定，可以增加 `download_timeout_seconds`
2. 视频可能已失效（被下架），程序会自动标记
3. 试试单次模式：`./bin/bili-downloader.exe --once`
4. 如果视频已被下架但仍想保存标题和描述，可以使用 metadata 模式

### Q4-1: 如何使用 metadata 模式只备份视频信息？

**A**: 在 `config.yaml` 中设置 `download_mode: "metadata"`，程序会将每个视频的标题、描述、UP主等文字信息保存为 `.txt` 文件，无需下载视频本身，也不需要安装 yt-dlp。特别适合：
- 视频已经被下架，想看标题知道是哪个视频
- 磁盘空间有限，先备份元数据以后再下载

### Q5: 收藏夹里的视频没有被下载

**A**: 检查以下几点：
1. 视频是否在 `start_date` 之后收藏的
2. 视频是否已经被下架（程序会标记为"已失效"）
3. 如果配置了 `monitored_favorites`，检查视频所在收藏夹是否在列表中
4. 查看日志输出，程序会打印跳过原因

### Q6: 如何只监控特定的收藏夹？

**A**: 在 `config.yaml` 中配置 `monitored_favorites`：

```yaml
# 只监控 AI 和 音乐 两个收藏夹
monitored_favorites:
  - "AI"
  - "音乐"
```

程序会模糊匹配，只要收藏夹名称包含这些关键词就会监控。

### Q7: 程序占用太多内存

**A**: 正常情况下内存占用很小（<100MB）。如果内存占用很高，可能是 SQLite 数据库文件较大，可以定期清理旧记录。

### Q8: 如何备份/迁移数据？

**A**: 直接复制 `videos.db` 文件即可。数据库是 SQLite 格式，跨平台通用。

### Q9: 可以同时运行多个实例吗？

**A**: 不建议。SQLite 不支持并发写入，可能导致数据损坏。

### Q10: 如何查看已下载的视频列表？

**A**: 使用 `--stats` 参数可以查看统计信息：
```bash
./bin/bili-downloader.exe --stats
```

### Q11: 程序支持 Linux/macOS 吗？

**A**: 支持，只需要重新编译：
```bash
# 在 src/ 目录下执行
# Linux
GOOS=linux go build -o ../bin/bili-downloader

# macOS
GOOS=darwin go build -o ../bin/bili-downloader
```

---

## 项目结构

```
bili-downloader/
├── src/                        # 源码目录
│   ├── main.go                 # 程序入口
│   ├── go.mod                  # Go 模块文件
│   ├── go.sum                  # 依赖锁定
│   ├── config/                 # 配置管理模块
│   │   └── config.go          # 读取和校验配置文件
│   ├── login/                  # 扫码登录模块
│   │   ├── qrlogin.go         # QR 码登录 API 调用
│   │   └── display.go         # 二维码生成和浏览器展示
│   ├── models/                 # 数据模型
│   │   └── video.go           # Video 和 Config 结构体定义
│   ├── storage/                # 数据库模块
│   │   └── db.go              # SQLite 数据库操作
│   ├── api/                    # B站 API 模块
│   │   └── bilibili.go        # 与 B站服务器通信
│   ├── downloader/             # 下载器模块
│   │   └── downloader.go      # 调用 yt-dlp 下载视频
│   └── monitor/               # 监控模块
│       └── monitor.go         # 定时检查收藏夹
├── bin/                        # 编译输出目录
├── config.yaml.example         # 配置文件模板
├── CHANGELOG.md                # 更新日志和审计记录
├── README.md                   # 本文档
└── LICENSE                     # MIT 许可证
```

### 各模块职责

| 模块 | 职责 |
|------|------|
| `config/` | 读取用户配置，验证配置项合法性 |
| `login/` | QR 码扫码登录，浏览器展示二维码 |
| `models/` | 定义数据结构（Video、Config） |
| `storage/` | 操作 SQLite 数据库，保存视频状态 |
| `api/` | 调用 B站 API 获取收藏夹信息 |
| `downloader/` | 调用 yt-dlp 下载视频 / 保存元数据到 txt |
| `monitor/` | 协调各模块，实现定时监控逻辑 |

---

## 命令行参数

| 参数 | 说明 |
|------|------|
| `--config <path>` | 指定配置文件路径（默认当前目录的 config.yaml） |
| `--once` | 仅执行一次同步，不持续监控 |
| `--stats` | 仅显示统计信息并退出 |

---

## 数据流向

```
┌──────────────────────────────────────────────────────────────┐
│                         用户配置                             │
│                    (config.yaml)                             │
│                                                               │
│   - favorite_folders: true/false                            │
│   - monitored_favorites: [] (监控哪些收藏夹)                 │
└─────────────────────────┬────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                      配置加载器                               │
│                    (config/config.go)                        │
└─────────────────────────┬────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                       B站 API                                 │
│                    (api/bilibili.go)                         │
│                                                               │
│   1. 验证 Cookie 是否有效                                     │
│   2. 获取收藏夹列表                                           │
│   3. 根据 monitored_favorites 过滤收藏夹                      │
│   4. 获取每个收藏夹的视频（含收藏夹名称）                       │
└─────────────────────────┬────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                       数据库                                  │
│                    (storage/db.go)                           │
│                                                               │
│   记录每个视频的状态：                                         │
│   - pending: 待下载                                           │
│   - downloaded: 已下载                                        │
│   - failed: 下载失败                                          │
│   - expired: 已失效（被下架）                                  │
│   - ignored: 日期隔离                                          │
│                                                               │
│   视频记录包含收藏夹信息：favorite_id, favorite_title         │
└─────────────────────────┬────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                      下载器                                   │
│                 (downloader/downloader.go)                   │
│                                                               │
│   调用 yt-dlp 下载视频到：                                     │
│   如果 favorite_folders=true:                                 │
│     {save_path}/{收藏夹名称}/{UP主}/{视频标题}.mp4            │
│   否则:                                                       │
│     {save_path}/{UP主}/{视频标题}.mp4                        │
└──────────────────────────────────────────────────────────────┘
```

---

## 注意事项

1. **仅供个人使用**：本工具仅用于个人备份收藏视频，请勿传播或用于商业用途
2. **尊重版权**：下载的视频仅供个人观看，请尊重 UP主 的版权
3. **推荐使用扫码登录**：Cookie 有有效期，扫码登录每次启动重新扫一次即可，不用担心过期
4. **合理设置间隔**：检查间隔不要太短（建议 >=30分钟），避免被 B站 风控
5. **网络环境**：确保网络稳定，大视频下载可能需要较长时间

---

## 更新日志

详见 [CHANGELOG.md](./CHANGELOG.md)

---

## 联系方式

如有问题或建议，欢迎提交 Issue。

---

**祝你使用愉快，再也不怕收藏的视频被删了！** 🎉
