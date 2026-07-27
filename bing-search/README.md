# bing-search

免 API Key 的 Bing 搜索 Go 库 + CLI 工具。通过模拟浏览器请求 + HTML 解析获取搜索结果，无需任何认证。

## 特性

- **零依赖** — 纯标准库，无第三方包
- **免认证** — 不需要 API Key，直接抓取 Bing 网页
- **多区域** — 支持 cn.bing.com / www.bing.com / global.bing.com
- **URL 解码** — 自动解码 Bing 包装链接，返回真实 URL
- **CLI + 库** — 既能命令行用，也能作为 Go 包导入

## 用法

### 命令行

```bash
go run main.go -q "Golang 并发编程" -n 10              # 基本搜索
go run main.go -q "golang channels" -json               # JSON 输出
go run main.go -q "test" -region global -lang en-US     # 指定区域和语言
go run main.go -q "Golang" -n 5 -page 2                 # 翻页（受限，见下方说明）
go run main.go -q "Golang" -n 5 -pages 3                # 多页合并（受限）
```

### 作为库

```go
import "github.com/user/bing-search/bing"

ctx := context.Background()
results, err := bing.Search(ctx, bing.SearchOptions{
    Query:  "Golang 并发编程",
    Count:  10,
    Region: bing.RegionCN,
})

for _, r := range results {
    fmt.Println(r.Title, r.URL, r.Snippet)
}
```

## 原理

1. 发送带完整浏览器 Header 的 HTTP 请求（关键：不设 Accept-Encoding，避免返回 JS 骨架页）
2. 用正则解析 Bing HTML 中 `<li class="b_algo">` 块
3. 提取标题、链接、摘要
4. 解码 Bing 包装 URL（`u=a1<base64>` → 真实 URL）

## 已知限制

- **翻页不可靠** — Bing 翻页依赖 JavaScript 执行，纯 HTTP 请求从服务器/CLI 发出时，Bing 会忽略 `first` 参数，始终返回第一页结果。这在 cn/www/global 三个域名上均已验证。如需可靠翻页，需配合 chromedp 等无头浏览器。`SearchMultiPage` 函数会尝试从 HTML 中提取下一页链接并跟随，但实际效果取决于运行环境。
- **结果数量不精确** — Bing 每次返回的结果数量不完全受 `count` 控制，实际数量取决于 Bing 服务端（实测约 9-10 条）。
- **反爬风险** — 频繁请求可能触发 Bing 反爬（返回 403 或验证码），建议加间隔。

## 参考

- [open-octo/octo-agent](https://github.com/open-octo/octo-agent) — browserGet + parseBingHTML 实现
- [odradekk/diting](https://github.com/odradekk/diting) — base64 URL 解码方案
- [VelariumAI/gorkbot](https://github.com/VelariumAI/gorkbot) — 类似的 Bing 爬虫实现
