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
		got := ParseListingSortQuery(url.Values{"sort": {"size"}, "order": {"desc"}})
		if got.Field != listingSortBySize || got.Order != listingSortDesc {
			t.Fatalf("unexpected parsed values: %+v", got)
		}
	})

	t.Run("invalid values fallback", func(t *testing.T) {
		got := ParseListingSortQuery(url.Values{"sort": {"invalid"}, "order": {"nope"}})
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
		"",
		"",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<table>") {
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
		"<h1>HEADER</h1>",
		"<p>FOOTER</p>",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
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
		"",
		"",
		ListingSortOptions{Field: listingSortByDate, Order: listingSortAsc},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}

	if !strings.Contains(html, "?sort=name&order=asc") {
		t.Fatal("expected name sort link")
	}
	if !strings.Contains(html, "?sort=date&order=desc") {
		t.Fatal("expected toggled date sort link")
	}
	if !strings.Contains(html, "?sort=size&order=asc") {
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
		"",
		"",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
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
		"",
		"",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
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
		"",
		"",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
	)
	if err != nil {
		t.Fatalf("RenderDirectoryListing failed: %v", err)
	}
	if !strings.Contains(childHTML, "href=\"../\"") {
		t.Fatal("non-root should contain parent link")
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
		"",
		"",
		ListingSortOptions{Field: listingSortByName, Order: listingSortAsc},
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
