package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/user/bing-search/bing"
)

func main() {
	query := flag.String("q", "", "搜索关键词")
	count := flag.Int("n", 5, "每页结果数量 (最多 50)")
	page := flag.Int("page", 1, "页码，从 1 开始")
	pages := flag.Int("pages", 1, "连续翻几页 (和 -page 互斥，>1 时自动翻页合并)")
	lang := flag.String("lang", "zh-CN", "语言偏好 (zh-CN, en-US, ...)")
	region := flag.String("region", "", "Bing 区域 (cn, global, ww)")
	timeout := flag.Int("timeout", 15, "超时秒数")
	jsonOut := flag.Bool("json", false, "输出 JSON 格式")
	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, "用法: bing-search -q \"搜索关键词\" [-n 5] [-page 1] [-pages 3] [-json]")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	opts := bing.SearchOptions{
		Query:  *query,
		Count:  *count,
		Lang:   *lang,
		Region: bing.Region(*region),
	}

	var results []bing.Result
	var err error

	if *pages > 1 {
		results, err = bing.SearchMultiPage(ctx, opts, *pages)
	} else {
		opts.Page = *page
		results, err = bing.Search(ctx, opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "搜索失败: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		return
	}

	if len(results) == 0 {
		fmt.Println("没有找到结果")
		return
	}

	for i, r := range results {
		fmt.Printf("[%d] %s\n", i+1, r.Title)
		fmt.Printf("    %s\n", r.URL)
		if r.Snippet != "" {
			fmt.Printf("    %s\n", r.Snippet)
		}
		fmt.Println()
	}
}
