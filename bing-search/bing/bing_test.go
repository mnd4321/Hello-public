package bing

import (
	"testing"
)

func TestParseBingHTML(t *testing.T) {
	html := `
<html><body>
<ol id="b_results">
<li class="b_algo" data-bm="0">
  <h2><a href="https://example.com/page1?bing.com/ck/a?test=1" target="_blank">Example Title 1</a></h2>
  <div class="b_caption"><p>This is the first result snippet text.</p></div>
</li>
<li class="b_algo" data-bm="1">
  <h2><a href="https://example.com/page2" h="ID=SERP,1234">Example Title 2</a></h2>
  <p class="b_lineclamp2">This is the second result snippet.</p>
</li>
<li class="b_algo" data-bm="2">
  <h2><a href="https://cn.bing.com/ck/a?u=a1aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20vdGVzdA&ntb=1" h="ID=SERP,5678">Encoded URL Title</a></h2>
  <p class="b_lineclamp3">This URL has base64 encoding.</p>
</li>
</ol>
</body></html>
`

	results := parseBingHTML(html, 10)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Title != "Example Title 1" {
		t.Errorf("title[0] = %q, want %q", results[0].Title, "Example Title 1")
	}
	if results[0].Snippet != "This is the first result snippet text." {
		t.Errorf("snippet[0] = %q", results[0].Snippet)
	}

	if results[1].Title != "Example Title 2" {
		t.Errorf("title[1] = %q, want %q", results[1].Title, "Example Title 2")
	}
	if results[1].URL != "https://example.com/page2" {
		t.Errorf("url[1] = %q, want %q", results[1].URL, "https://example.com/page2")
	}

	if results[2].URL != "https://www.example.com/test" {
		t.Errorf("url[2] = %q, want %q", results[2].URL, "https://www.example.com/test")
	}

	results2 := parseBingHTML(html, 1)
	if len(results2) != 1 {
		t.Fatalf("expected 1 result with max=1, got %d", len(results2))
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<b>hello</b>", "hello"},
		{"&amp; &lt; &gt; &quot; &#39;", "& < > \" '"},
		{"<a href=\"http://x.com\">link</a>", "link"},
		{"  spaces  ", "spaces"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDecodeBingURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com", "https://example.com"},
		{"https://cn.bing.com/ck/a?u=a1aHR0cHM6Ly93d3cuZXhhbXBsZS5jb20vdGVzdA&ntb=1", "https://www.example.com/test"},
		{"https://cn.bing.com/ck/a?u=a1!!!invalid!!!&ntb=1", "https://cn.bing.com/ck/a?u=a1!!!invalid!!!&ntb=1"},
	}
	for _, tt := range tests {
		got := decodeBingURL(tt.input)
		if got != tt.want {
			t.Errorf("decodeBingURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildEndpoint(t *testing.T) {
	tests := []struct {
		region Region
		want   string
	}{
		{RegionCN, "https://cn.bing.com/search"},
		{RegionGlobal, "https://www.bing.com/search"},
		{RegionWW, "https://global.bing.com/search"},
		{"", "https://cn.bing.com/search"},
	}
	for _, tt := range tests {
		got := buildEndpoint(tt.region)
		if got != tt.want {
			t.Errorf("buildEndpoint(%q) = %q, want %q", tt.region, got, tt.want)
		}
	}
}
