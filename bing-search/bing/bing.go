// Package bing 提供免 API Key 的 Bing 搜索功能
// 通过模拟浏览器请求 + HTML 解析获取搜索结果，无需任何认证
//
// 参考项目:
//   - open-octo/octo-agent (browserGet + parseBingHTML)
//   - odradekk/diting (base64 URL 解码)
package bing

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result 表示一条搜索结果
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Region 表示 Bing 区域，决定使用哪个域名
type Region string

const (
	RegionCN     Region = "cn"     // cn.bing.com (默认)
	RegionGlobal Region = "global" // www.bing.com
	RegionWW     Region = "ww"     // global.bing.com
)

// SearchOptions 搜索参数
type SearchOptions struct {
	Query  string // 搜索关键词 (必填)
	Count  int    // 每页数量，最多 50，默认 10
	Page   int    // 页码，从 1 开始，默认 1
	Lang   string // Accept-Language，默认 zh-CN
	Region Region // Bing 区域，默认 cn
}

// Search 执行一次 Bing 搜索，返回结果列表
func Search(ctx context.Context, opts SearchOptions) ([]Result, error) {
	if opts.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if opts.Count <= 0 {
		opts.Count = 10
	}
	if opts.Count > 50 {
		opts.Count = 50
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.Lang == "" {
		opts.Lang = "zh-CN"
	}

	endpoint := buildEndpoint(opts.Region)
	first := (opts.Page-1)*opts.Count + 1
	u := endpoint + "?q=" + url.QueryEscape(opts.Query) +
		"&count=" + strconv.Itoa(opts.Count) +
		"&first=" + strconv.Itoa(first)

	body, err := browserGet(ctx, u, opts.Lang, 2)
	if err != nil {
		return nil, fmt.Errorf("bing: %w", err)
	}

	return parseBingHTML(body, opts.Count), nil
}

// SearchMultiPage 连续搜索多页，合并返回所有结果
//
// 注意: Bing 翻页依赖 JavaScript 执行。纯 HTTP 请求环境下
// （服务器/CLI），Bing 可能忽略 first 参数，始终返回第一页结果。
// 如需可靠翻页，建议配合 chromedp 等无头浏览器使用。
//
// 当前实现会尝试从第1页 HTML 中提取下一页链接并跟随，
// 但成功率取决于 Bing 的反爬策略和请求来源 IP。
//
// pages 指定要翻几页（如 pages=3 则获取第1~3页）
func SearchMultiPage(ctx context.Context, opts SearchOptions, pages int) ([]Result, error) {
	if pages <= 0 {
		pages = 1
	}
	if pages > 10 {
		pages = 10
	}

	var all []Result
	nextURL := ""

	for p := 1; p <= pages; p++ {
		var body string
		var err error

		if p == 1 {
			if opts.Count <= 0 {
				opts.Count = 10
			}
			if opts.Count > 50 {
				opts.Count = 50
			}
			if opts.Lang == "" {
				opts.Lang = "zh-CN"
			}
			endpoint := buildEndpoint(opts.Region)
			first := (opts.Page-1)*opts.Count + 1
			u := endpoint + "?q=" + url.QueryEscape(opts.Query) +
				"&count=" + strconv.Itoa(opts.Count) +
				"&first=" + strconv.Itoa(first)
			body, err = browserGet(ctx, u, opts.Lang, 2)
		} else {
			if nextURL == "" {
				break
			}
			fullURL := nextURL
			if strings.HasPrefix(nextURL, "/") {
				ep := buildEndpoint(opts.Region)
				u, _ := url.Parse(ep)
				fullURL = u.Scheme + "://" + u.Host + nextURL
			}
			body, err = browserGet(ctx, fullURL, opts.Lang, 2)
		}

		if err != nil {
			if p == 1 {
				return nil, err
			}
			break
		}

		results := parseBingHTML(body, opts.Count)
		if len(results) == 0 {
			break
		}
		all = append(all, results...)

		nextURL = extractNextPageLink(body)
	}
	return all, nil
}

// extractNextPageLink 从 Bing HTML 中提取下一页链接
func extractNextPageLink(html string) string {
	re := regexp.MustCompile(`(?s)b_pag.*?</nav>`)
	block := re.FindString(html)
	if block == "" {
		return ""
	}
	linkRE := regexp.MustCompile(`href="(/search\?[^"]*first=\d+[^"]*(?:PERE|PORE)[^"]*?)"`)
	m := linkRE.FindStringSubmatch(block)
	if m == nil {
		fallback := regexp.MustCompile(`href="(/search\?[^"]*first=\d+[^"*]*)"`)
		m = fallback.FindStringSubmatch(block)
	}
	if m != nil {
		return strings.ReplaceAll(m[1], "&amp;", "&")
	}
	return ""
}

// buildEndpoint 根据区域返回 Bing 域名
func buildEndpoint(region Region) string {
	switch region {
	case RegionGlobal:
		return "https://www.bing.com/search"
	case RegionWW:
		return "https://global.bing.com/search"
	default:
		return "https://cn.bing.com/search"
	}
}

// ───────────────────── 浏览器请求 ─────────────────────

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
}

// browserGet 模拟浏览器发送 GET 请求，返回 HTML 原文
// 关键: 故意不发送 Accept-Encoding 头，避免 Bing 返回 JS 骨架页
func browserGet(ctx context.Context, target string, lang string, followRedirects int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}

	ua := userAgents[rand.Intn(len(userAgents))]
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", lang+",zh;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	for hops := 0; resp.StatusCode >= 300 && resp.StatusCode < 400 && hops < followRedirects; hops++ {
		loc := resp.Header.Get("Location")
		if loc == "" {
			break
		}
		_ = resp.Body.Close()

		if strings.HasPrefix(loc, "/") {
			u, _ := url.Parse(target)
			loc = u.Scheme + "://" + u.Host + loc
		}

		nextReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
		nextReq.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
		nextReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		nextReq.Header.Set("Accept-Language", lang+",zh;q=0.9,en;q=0.8")
		nextReq.Header.Set("Sec-Fetch-Dest", "document")
		nextReq.Header.Set("Sec-Fetch-Mode", "navigate")
		nextReq.Header.Set("Sec-Fetch-Site", "none")
		nextReq.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err = client.Do(nextReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		target = loc
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// ───────────────────── HTML 解析 ─────────────────────

var bingBlockRE = regexp.MustCompile(`(?s)<li[^>]*class="b_algo"[^>]*>(.*?)</li>`)
var bingTitleRE = regexp.MustCompile(`(?s)<h2[^>]*>.*?<a[^>]*href="(https?://[^"]+)"[^>]*>(.*?)</a>`)
var bingLineclampRE = regexp.MustCompile(`(?s)<p[^>]*class="b_lineclamp\d*"[^>]*>(.*?)</p>`)
var bingCaptionRE = regexp.MustCompile(`(?s)<div[^>]*class="b_caption"[^>]*>.*?<p[^>]*>(.*?)</p>`)

func parseBingHTML(body string, max int) []Result {
	blocks := bingBlockRE.FindAllStringSubmatch(body, -1)
	out := make([]Result, 0, len(blocks))

	for _, b := range blocks {
		if len(out) >= max {
			break
		}

		titleMatch := bingTitleRE.FindStringSubmatch(b[1])
		if titleMatch == nil {
			continue
		}

		realURL := decodeBingURL(titleMatch[1])
		title := stripHTML(titleMatch[2])

		snippet := ""
		if m := bingLineclampRE.FindStringSubmatch(b[1]); m != nil {
			snippet = stripHTML(m[1])
		} else if m := bingCaptionRE.FindStringSubmatch(b[1]); m != nil {
			snippet = stripHTML(m[1])
		}

		out = append(out, Result{Title: title, URL: realURL, Snippet: snippet})
	}

	return out
}

// decodeBingURL 解码 Bing 包装链接
// 格式: bing.com/ck/a?...&u=a1<URL-safe-base64>&ntb=1
func decodeBingURL(wrapped string) string {
	if !strings.Contains(wrapped, "bing.com/ck/") {
		return wrapped
	}

	u, err := url.Parse(wrapped)
	if err != nil {
		return wrapped
	}

	uVal := u.Query().Get("u")
	if uVal == "" || !strings.HasPrefix(uVal, "a1") {
		return wrapped
	}

	payload := uVal[2:]
	if pad := len(payload) % 4; pad != 0 {
		payload += strings.Repeat("=", 4-pad)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return wrapped
	}

	return string(decoded)
}

// ───────────────────── 工具函数 ─────────────────────

var tagRE = regexp.MustCompile(`<[^>]+>`)

var htmlEntities = strings.NewReplacer(
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
	"&#x27;", "'",
	"&nbsp;", " ",
	"&ensp;", " ",
	"&emsp;", "  ",
	"&thinsp;", " ",
	"&#183;", "·",
	"&hellip;", "…",
	"&mdash;", "—",
	"&ndash;", "–",
)

func stripHTML(s string) string {
	s = tagRE.ReplaceAllString(s, "")
	s = htmlEntities.Replace(s)
	return strings.TrimSpace(s)
}
