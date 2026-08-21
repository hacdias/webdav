package lib

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/webdav"
)

func TestParseListingSortQuery(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		got := ParseListingSortQuery(url.Values{})
		if got.Field != listingSortByName || got.Order != listingSortAsc {
			t.Fatalf("unexpected defaults: %+v", got)
		}
	})

	t.Run("valid values", func(t *testing.T) {
		got := ParseListingSortQuery(url.Values{"C": {"S"}, "O": {"D"}})
		if got.Field != listingSortBySize || got.Order != listingSortDesc {
			t.Fatalf("unexpected parsed values: %+v", got)
		}
	})

	t.Run("invalid values fallback", func(t *testing.T) {
		got := ParseListingSortQuery(url.Values{"C": {"invalid"}, "O": {"nope"}})
		if got.Field != listingSortByName || got.Order != listingSortAsc {
			t.Fatalf("unexpected fallback values: %+v", got)
		}
	})
}

func TestRenderDirectoryListingBasic(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0o644)
	_ = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755)

	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, `<table id="list">`) {
		t.Fatal("expected basic html table output")
	}
	if !strings.Contains(html, "Index of /") {
		t.Fatal("expected index heading")
	}
	if strings.Contains(html, "📁") || strings.Contains(html, "📄") {
		t.Fatal("expected simplified output without icons")
	}
}

func TestRenderDirectoryListingHeaderFooter(t *testing.T) {
	tmpDir := t.TempDir()
	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{Header: "<h1>HEADER</h1>", Footer: "<p>FOOTER</p>", ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "<h1>HEADER</h1>") || !strings.Contains(html, "<p>FOOTER</p>") {
		t.Fatal("expected header/footer in output")
	}
}

func TestRenderDirectoryListingSortLinks(t *testing.T) {
	tmpDir := t.TempDir()
	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByDate, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "?C=N&O=A") {
		t.Fatal("expected name sort link")
	}
	if !strings.Contains(html, "?C=M&O=D") {
		t.Fatal("expected toggled date sort link")
	}
	if !strings.Contains(html, "?C=S&O=A") {
		t.Fatal("expected size sort link")
	}
}

func TestRenderDirectoryListingEscapingAndLinks(t *testing.T) {
	tmpDir := t.TempDir()
	filename := "a b&c<d>.txt"
	_ = os.WriteFile(filepath.Join(tmpDir, filename), []byte("x"), 0o644)
	_ = os.Mkdir(filepath.Join(tmpDir, "mydir"), 0o755)

	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "a b&amp;c&lt;d&gt;.txt") {
		t.Fatal("expected escaped display name")
	}
	if !strings.Contains(html, "href=\"a%20b&c%3Cd%3E.txt\"") {
		t.Fatal("expected url-escaped href")
	}
	if !strings.Contains(html, "href=\"mydir/\"") {
		t.Fatal("expected trailing slash for directory links")
	}
}

func TestRenderDirectoryListingParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Mkdir(filepath.Join(tmpDir, "child"), 0o755)

	rootHTML, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if strings.Contains(rootHTML, "href=\"../\"") {
		t.Fatal("root should not contain parent link")
	}

	childHTML, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/child",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if !strings.Contains(childHTML, "href=\"../\"") {
		t.Fatal("non-root should contain parent link")
	}
	if !strings.Contains(childHTML, `<td colspan="2" class="link"><a href="../">../</a></td>`) {
		t.Fatal("parent link should use fancyindex-style link cell")
	}

	hiddenChildHTML, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/child",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{HideParentDir: true, ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if strings.Contains(hiddenChildHTML, "href=\"../\"") {
		t.Fatal("non-root should not contain parent link when hideParentDir is true")
	}
}

func TestRenderDirectoryListingShowPath(t *testing.T) {
	tmpDir := t.TempDir()

	shown, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if !strings.Contains(shown, "<h1>Index of") {
		t.Fatal("expected default title when showPath is true")
	}

	hidden, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: false},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if strings.Contains(hidden, "<h1>Index of") {
		t.Fatal("expected no title when showPath is false and no custom header is set")
	}

	withHeader, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{Header: "<h1>Custom</h1>", ShowPath: false},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if !strings.Contains(withHeader, "<h1>Custom</h1>") {
		t.Fatal("custom header should still render regardless of showPath")
	}
}

func TestRenderDirectoryListingSortByName(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "zebra.txt"), []byte("z"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "apple.txt"), []byte("a"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "banana.txt"), []byte("b"), 0o644)

	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	apple := strings.Index(html, "apple.txt")
	banana := strings.Index(html, "banana.txt")
	zebra := strings.Index(html, "zebra.txt")
	if !(apple < banana && banana < zebra) {
		t.Fatal("expected alphabetical order")
	}
}

func TestRenderDirectoryListingFancyindexClasses(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0o644)

	html, err := RenderDirectoryListing(
		context.Background(),
		webdav.Dir(tmpDir),
		"/",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
		BrowserListing{ShowPath: true},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, `<th colspan="2"><a href="?C=N&O=D">Name ↑</a></th>`) {
		t.Fatal("expected filename header to span two columns")
	}
	if !strings.Contains(html, `<th class="size"><a href="?C=S&O=A">Size</a></th>`) {
		t.Fatal("expected size header class")
	}
	if !strings.Contains(html, `<th class="date"><a href="?C=M&O=A">Last modified</a></th>`) {
		t.Fatal("expected date header class")
	}
	if !strings.Contains(html, `<td colspan="2" class="link"><a href="file.txt">file.txt</a></td>`) {
		t.Fatal("expected file link cell to use fancyindex-style class")
	}
	if !strings.Contains(html, `<td class="size">7 B</td>`) {
		t.Fatal("expected size cell class")
	}
	if !strings.Contains(html, `<td class="date">`) {
		t.Fatal("expected date cell class")
	}
	if strings.Contains(html, `td class="name"`) || strings.Contains(html, `a class="link"`) {
		t.Fatal("expected old browser listing classes to be removed")
	}
}

func TestSortFileEntriesByDateAndSize(t *testing.T) {
	now := time.Now()
	files := []FileEntry{
		{Name: "a", Size: 10, ModTime: now.Add(-time.Hour)},
		{Name: "b", Size: 30, ModTime: now.Add(-3 * time.Hour)},
		{Name: "c", Size: 20, ModTime: now.Add(-2 * time.Hour)},
	}

	sortFileEntries(files, ListingSortOptions{Field: listingSortByDate, Order: listingSortAsc})
	if files[0].Name != "b" || files[2].Name != "a" {
		t.Fatal("expected ascending date order")
	}

	sortFileEntries(files, ListingSortOptions{Field: listingSortByDate, Order: listingSortDesc})
	if files[0].Name != "a" || files[2].Name != "b" {
		t.Fatal("expected descending date order")
	}

	sortFileEntries(files, ListingSortOptions{Field: listingSortBySize, Order: listingSortAsc})
	if files[0].Name != "a" || files[2].Name != "b" {
		t.Fatal("expected ascending size order")
	}

	sortFileEntries(files, ListingSortOptions{Field: listingSortBySize, Order: listingSortDesc})
	if files[0].Name != "b" || files[2].Name != "a" {
		t.Fatal("expected descending size order")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
	}

	for _, tc := range tests {
		if got := formatSize(tc.size); got != tc.expected {
			t.Fatalf("formatSize(%d)=%q want %q", tc.size, got, tc.expected)
		}
	}
}
