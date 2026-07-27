# Bing 搜索接口研究与 Go 实现

## 1. 背景

调研 Bing 搜索的公开接口方案，目标是找到**免 API Key、纯 HTTP 请求**的搜索方式，并用 Go 实现为可用的库 + CLI 工具。

## 2. 调研结果

### 2.1 Bing 搜索接口类型

| 类型 | 方式 | 需要 Key | 特点 |
|---|---|---|---|
| **官方 API** | `api.bing.microsoft.com/v7.0/search` | ✅ 必须（Azure 订阅） | 结构化 JSON，稳定可靠，有配额限制 |
| **网页抓取** | 直接请求 `bing.com/search` | ❌ 不需要 | 解析 HTML，无配额限制，但有反爬风险 |
| **RSS 格式** | `bing.com/search?q=xxx&format=rss` | ❌ 不需要 | XML 结构，比 HTML 更好解析，但数据量少 |
| **逆向 Chat** | EdgeGPT 等 | ❌ 不需要 | 通过逆向 Bing Chat 会话接口获取 AI 回答 |

本项目选择**网页抓取**方案——零依赖、零认证、纯标准库。

### 2.2 GitHub 上的相关开源项目

通过 GitHub 代码搜索，找到以下 Go 语言实现的 Bing 爬虫：

| 仓库 | 关键实现 | 技巧 |
|---|---|---|
| [open-octo/octo-agent](https://github.com/open-octo/octo-agent) | `searchBing()` + `parseBingHTML()` | **不设 Accept-Encoding**，避免返回 JS 骨架页 |
| [odradekk/diting](https://github.com/odradekk/diting) | `internal/search/bing/bing.go` | Bing 包装 URL 的 base64 解码 |
| [VelariumAI/gorkbot](https://github.com/VelariumAI/gorkbot) | `scrapling.go` | 类似 HTML 正则解析 |
| [AnikHasibul/bing](https://github.com/AnikHasibul/bing) | `bing.go` | 最早期的 Go 实现之一 |
| [FuturixAI-and-Quantum-Works/Google-Serp](https://github.com/FuturixAI-and-Quantum-Works/Google-Serp) | `search/bing_search.go` | 支持多搜索引擎 |

此外还有非 Go 语言但值得参考的项目：

| 仓库 | 语言 | ⭐ | 说明 |
|---|---|---|---|
| [karust/openserp](https://github.com/karust/openserp) | Python | 1160 | 自托管 SERP API，支持 Bing/Google/Baidu |
| [vikiboss/60s](https://github.com/vikiboss/60s) | TypeScript | 5558 | 综合 API 集合，含 Bing 搜索/翻译/壁纸 |
| [acheong08/EdgeGPT](https://github.com/acheong08/EdgeGPT) | Python | 7858 | Bing Chat 逆向 |

### 2.3 核心技术要点

通过分析上述项目源码，总结出 Bing 网页抓取的三个关键技巧：

#### 技巧一：不设 Accept-Encoding（最重要）

```go
// ❌ 错误：设置了 Accept-Encoding
req.Header.Set("Accept-Encoding", "gzip, deflate")

// ✅ 正确：故意不设置
// Go 的 http.Client 会自动处理 gzip 解压，
// 手动设置反而会让 Bing 返回 ~39KB 的 JS 骨架页
// 而非 ~120KB 的真实结果页
```

这是 open-octo/octo-agent 在注释中明确标注的关键发现：

> CRITICAL: we deliberately DO NOT send an Accept-Encoding header. If we announce gzip support to Bing, Bing returns a ~39 KB JavaScript skeleton page instead of the ~120 KB real results page.

#### 技巧二：完整的浏览器 Header

```go
req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...")
req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
req.Header.Set("Sec-Fetch-Dest", "document")
req.Header.Set("Sec-Fetch-Mode", "navigate")
req.Header.Set("Sec-Fetch-Site", "none")
req.Header.Set("Upgrade-Insecure-Requests", "1")
```

缺少 `Sec-Fetch-*` 系列头也会导致 Bing 返回精简页面。

#### 技巧三：解码 Bing 包装 URL

Bing 出站链接格式：
```
https://cn.bing.com/ck/a?u=a1aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20&ntb=1
```

其中 `u` 参数 = `a1` 前缀 + URL-safe Base64 编码的真实 URL（无 padding）：

```go
func decodeBingURL(wrapped string) string {
    if !strings.Contains(wrapped, "bing.com/ck/") {
        return wrapped
    }
    u, _ := url.Parse(wrapped)
    uVal := u.Query().Get("u")        // "a1aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20"
    payload := uVal[2:]               // "aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20"
    // 补齐 padding
    if pad := len(payload) % 4; pad != 0 {
        payload += strings.Repeat("=", 4-pad)
    }
    decoded, _ := base64.URLEncoding.DecodeString(payload)
    return string(decoded)            // "https://www.example.com"
}
```

## 3. HTML 结构分析

### 3.1 搜索结果结构

```html
<ol id="b_results">
  <li class="b_algo">                    ← 每条结果的容器
    <h2>
      <a href="URL" h="ID=SERP,...">     ← 标题 + 链接
        标题文本
      </a>
    </h2>
    <div class="b_caption">              ← 摘要区域
      <p class="b_lineclamp2">           ← 摘要文本（优先匹配）
        摘要内容...
      </p>
    </div>
  </li>
  <li class="b_algo">
    <!-- 下一条结果 -->
  </li>
</ol>
```

### 3.2 翻页结构

```html
<nav class="b_pag" aria-label="更多搜索结果">
  <ul class="sb_pagF">
    <li>
      <a class="sb_pagS sb_pagS_bp">1</a>  ← 当前页（无 href）
    </li>
    <li>
      <a href="/search?q=...&FPIG=xxx&first=11&FORM=PERE">2</a>  ← 下一页
    </li>
    <li>
      <a href="/search?q=...&FPIG=xxx&first=21&FORM=PERE1">3</a>
    </li>
  </ul>
</nav>
```

翻页链接特征：包含 `FPIG` 会话参数 + `FORM=PERE` 标记 + `first` 偏移量。

### 3.3 正则提取

```go
// 匹配结果块
var bingBlockRE = regexp.MustCompile(`(?s)<li[^>]*class="b_algo"[^>]*>(.*?)</li>`)

// 从块中提取标题+链接
var bingTitleRE = regexp.MustCompile(`(?s)<h2[^>]*>.*?<a[^>]*href="(https?://[^"]+)"[^>]*>(.*?)</a>`)

// 摘要：优先 b_lineclamp，其次 b_caption
var bingLineclampRE = regexp.MustCompile(`(?s)<p[^>]*class="b_lineclamp\d*"[^>]*>(.*?)</p>`)
var bingCaptionRE   = regexp.MustCompile(`(?s)<div[^>]*class="b_caption"[^>]*>.*?<p[^>]*>(.*?)</p>`)
```

## 4. 翻页限制（实测验证）

### 4.1 测试环境

- 服务器 IP（海外数据中心）
- 测试域名：cn.bing.com / www.bing.com / global.bing.com
- 测试参数：`first`、`count`、`form=QBLH`、`form=QBRE`、`start`

### 4.2 测试结论

**所有翻页参数均被忽略，Bing 始终返回第一页结果。**

| 测试项 | 结果 |
|---|---|
| `first=11` 单独使用 | ❌ 返回第一页 |
| `first=11&form=QBLH` | ❌ 返回第一页 |
| `first=11&form=QBRE` | ❌ 返回第一页 |
| `first=11&FPIG=xxx&FORM=PERE`（从 HTML 提取的完整链接） | ❌ 返回第一页 |
| `start=10` | ❌ 返回第一页 |
| `count=20` | ❌ 返回约 9 条（忽略 count） |
| 移动端 UA | ❌ 同样不翻页 |
| 带 Cookie + Referer | ❌ 同样不翻页 |
| RSS 格式 `format=rss&first=11` | ❌ 同样不翻页 |

### 4.3 原因分析

Bing 的翻页逻辑由**客户端 JavaScript** 处理。`first` 参数在浏览器中通过 JS 发起 AJAX 请求才生效，纯 HTTP GET 请求中 Bing 服务端会忽略该参数。

这意味着：
- ✅ 浏览器中手动访问 `bing.com/search?q=xxx&first=11` 能翻页（JS 处理）
- ❌ 服务器端 Go/Python/Node 发出的 HTTP 请求不能翻页

### 4.4 可行的翻页方案

| 方案 | 复杂度 | 可靠性 |
|---|---|---|
| **chromedp**（Go 无头浏览器） | 中 | ✅ 高 |
| **playwright-go** | 中 | ✅ 高 |
| **Selenium + Go binding** | 高 | ✅ 高 |
| **纯 HTTP（本项目）** | 低 | ❌ 只能第一页 |

## 5. 最终实现

### 5.1 项目结构

```
bing-search/
├── main.go              # CLI 入口
├── go.mod               # Go 模块定义
├── README.md
└── bing/
    ├── bing.go          # 核心库：Search / SearchMultiPage / HTML解析
    └── bing_test.go     # 单元测试
```

### 5.2 API 设计

```go
// 搜索参数
type SearchOptions struct {
    Query  string  // 搜索关键词（必填）
    Count  int     // 每页数量，最多 50，默认 10
    Page   int     // 页码，从 1 开始，默认 1
    Lang   string  // Accept-Language，默认 zh-CN
    Region Region  // cn / global / ww
}

// 搜索结果
type Result struct {
    Title   string
    URL     string
    Snippet string
}

// 单页搜索
func Search(ctx context.Context, opts SearchOptions) ([]Result, error)

// 多页搜索（翻页受限，见上文说明）
func SearchMultiPage(ctx context.Context, opts SearchOptions, pages int) ([]Result, error)
```

### 5.3 CLI 用法

```bash
# 基本搜索
go run main.go -q "Golang 并发编程" -n 10

# JSON 输出
go run main.go -q "golang channels" -json

# 指定区域和语言
go run main.go -q "test" -region global -lang en-US

# 翻页（受限）
go run main.go -q "Golang" -n 5 -page 2
go run main.go -q "Golang" -n 5 -pages 3
```

### 5.4 实测效果

```
$ go run main.go -q "Golang 并发编程" -n 5

[1] The Go Programming Language
    https://golang.google.cn/
    Get Started Playground Tour Stack Overflow Help Packages...

[2] Go 语言教程 | 菜鸟教程
    https://www.runoob.com/go/go-tutorial.html
    Go 语言用途 Go 语言被设计成一门应用于搭载 Web 服务器...

[3] Go下载 - GoLang文档
    https://www.golangdev.cn/zh-cn/guide/download.html
    有关这些服务的隐私信息，请参阅...

[4] 首页 | Golang 中文学习文档
    https://golang.halfiisland.com/
    Go中文学习文档站，为Go语言爱好者提供丰富的学习资源...

[5] Download and install - The Go Programming Language
    https://go.dev/doc/install
    Documentation Download and install...
```

### 5.5 单测结果

```
=== RUN   TestParseBingHTML    --- PASS
=== RUN   TestStripHTML        --- PASS
=== RUN   TestDecodeBingURL    --- PASS
=== RUN   TestBuildEndpoint    --- PASS
PASS  ok  github.com/user/bing-search/bing  0.002s
```

## 6. 局限性与改进方向

### 当前局限

| 局限 | 说明 | 影响 |
|---|---|---|
| 翻页不可靠 | Bing 翻页需 JS 执行 | 只能获取第一页（约 9-10 条） |
| 结果数量不精确 | `count` 参数被忽略 | 始终返回约 9 条 |
| HTML 结构不稳定 | Bing 可能随时改版 | 正则可能失效 |
| 反爬风险 | 高频请求会触发验证码 | 不适合批量抓取 |

### 改进方向

1. **支持无头浏览器翻页** — 集成 chromedp，支持可靠翻页
2. **多搜索引擎** — 增加 Google、DuckDuckGo、Yandex 等后端
3. **结果缓存** — LRU 缓存减少重复请求
4. **代理支持** — 支持 HTTP/SOCKS5 代理轮换
5. **结构化数据提取** — 除标题/摘要外，提取日期、作者、图片等

## 7. 项目地址

- **GitHub**: [mnd4321/Hello-public/bing-search](https://github.com/mnd4321/Hello-public/tree/main/bing-search)
