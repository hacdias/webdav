package lib

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// makeListingFS builds an in-memory file system from a map of names to
// contents, where a name ending in "/" is a directory. The in-memory file
// system accepts names that some platforms reject (e.g. ":" or "<" on
// Windows), so hostile names can be tested everywhere.
func makeListingFS(t *testing.T, m map[string]string) webdav.FileSystem {
	ctx := context.Background()
	fs := webdav.NewMemFS()

	for name, data := range m {
		if strings.HasSuffix(name, "/") {
			require.NoError(t, fs.Mkdir(ctx, "/"+strings.TrimSuffix(name, "/"), 0o755))
			continue
		}

		f, err := fs.OpenFile(ctx, "/"+name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		_, err = f.Write([]byte(data))
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	return fs
}

func renderListing(t *testing.T, fs webdav.FileSystem, dirPath string, sorting ListingSortOptions, listing Listing) string {
	html, err := RenderListing(context.Background(), fs, dirPath, sorting, listing)
	require.NoError(t, err)
	return html
}

var sortByNameAsc = ListingSortOptions{Field: listingSortByName, Order: listingSortAsc}

func TestParseListingSortQuery(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		got := ParseListingSortQuery(url.Values{})
		if got.Field != listingSortByDate || got.Order != listingSortDesc {
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
		if got.Field != listingSortByDate || got.Order != listingSortDesc {
			t.Fatalf("unexpected fallback values: %+v", got)
		}
	})
}

func TestRenderDirectoryListingBasic(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"file1.txt": "content1",
		"subdir/":   "",
	})

	html := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})

	if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, `<table id="list">`) {
		t.Fatal("expected basic html table output")
	}
	if !strings.Contains(html, "<title>Index of /</title>") || !strings.Contains(html, "<h1>Index of /</h1>") {
		t.Fatal("expected index title and heading")
	}
	if !strings.Contains(html, `<a href="subdir/" title="subdir">subdir/</a>`) {
		t.Fatal("expected directory entry")
	}
	if !strings.Contains(html, `<a href="file1.txt" title="file1.txt">file1.txt</a>`) {
		t.Fatal("expected file entry")
	}
	if !strings.HasSuffix(html, "</tbody>\n</table></body>\n</html>") {
		t.Fatal("expected default document closing")
	}
}

func TestRenderDirectoryListingHeaderFooter(t *testing.T) {
	fs := makeListingFS(t, nil)
	html := renderListing(t, fs, "/", sortByNameAsc, Listing{
		Header:   "<!DOCTYPE html><html><head><title>Custom</title></head><body><h1>HEADER</h1>",
		Footer:   "<p>FOOTER</p></body></html>",
		ShowPath: true,
	})

	if !strings.HasPrefix(html, "<!DOCTYPE html><html><head><title>Custom</title></head><body><h1>HEADER</h1>") {
		t.Fatal("expected header to be rendered unescaped at the start")
	}
	if !strings.HasSuffix(html, "</table><p>FOOTER</p></body></html>") {
		t.Fatal("expected footer to be rendered unescaped at the end")
	}
	if strings.Count(html, "<html>") != 1 || strings.Count(html, "</html>") != 1 {
		t.Fatal("expected exactly one html open/close tag when header/footer own the shell")
	}
	if strings.Count(html, "<body>") != 1 || strings.Count(html, "</body>") != 1 {
		t.Fatal("expected exactly one body open/close tag when header/footer own the shell")
	}
	if strings.Contains(html, `<html lang="en">`) {
		t.Fatal("expected default shell to be skipped when a custom header is set")
	}
}

func TestRenderDirectoryListingSortLinks(t *testing.T) {
	fs := makeListingFS(t, nil)
	html := renderListing(t, fs, "/", ListingSortOptions{Field: listingSortByDate, Order: listingSortAsc}, Listing{ShowPath: true})

	if !strings.Contains(html, `href="?C=N&amp;O=A"`) {
		t.Fatal("expected name sort link")
	}
	if !strings.Contains(html, `href="?C=M&amp;O=D"`) {
		t.Fatal("expected toggled date sort link")
	}
	if !strings.Contains(html, `href="?C=S&amp;O=A"`) {
		t.Fatal("expected size sort link")
	}
}

func TestRenderDirectoryListingEscapingAndLinks(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"a b&c<d>.txt": "x",
		"mydir/":       "",
	})

	html := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})

	if !strings.Contains(html, `title="a b&amp;c&lt;d&gt;.txt">a b&amp;c&lt;d&gt;.txt</a>`) {
		t.Fatal("expected escaped display name")
	}
	if !strings.Contains(html, `href="a%20b&amp;c%3Cd%3E.txt"`) {
		t.Fatal("expected url-escaped href")
	}
	if !strings.Contains(html, `href="mydir/"`) {
		t.Fatal("expected trailing slash for directory links")
	}
}

func TestRenderDirectoryListingHostileNames(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"javascript:alert(window.location)":  "x",
		"javascript:alert(document.cookie)/": "",
		"<script>alert(1)<":                  "x",
		`"><img src=x onerror=alert(1)>`:     "x",
		"?C=N&O=A#frag":                      "x",
	})

	html := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})

	t.Run("URL schemes", func(t *testing.T) {
		require.NotContains(t, html, `href="javascript:`)
		require.Contains(t, html, `href="./javascript:alert%28window.location%29"`)
		require.Contains(t, html, `href="./javascript:alert%28document.cookie%29/"`)
	})

	t.Run("Markup", func(t *testing.T) {
		require.NotContains(t, html, "<script>")
		require.NotContains(t, html, "<img")
		require.Contains(t, html, `title="&lt;script&gt;alert(1)&lt;">&lt;script&gt;alert(1)&lt;</a>`)
		require.Contains(t, html, `title="&#34;&gt;&lt;img src=x onerror=alert(1)&gt;"`)
	})

	t.Run("Query and fragment", func(t *testing.T) {
		require.Contains(t, html, `href="%3FC=N&amp;O=A%23frag"`)
	})
}

func TestRenderDirectoryListingEscapesPath(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"<b>dir/": "",
	})

	html := renderListing(t, fs, "/<b>dir", sortByNameAsc, Listing{ShowPath: true})

	require.NotContains(t, html, "<b>")
	require.Contains(t, html, "<title>Index of /&lt;b&gt;dir/</title>")
	require.Contains(t, html, "<h1>Index of /&lt;b&gt;dir/</h1>")
}

func TestRenderDirectoryListingParentDirectory(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"child/": "",
	})

	rootHTML := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})
	if strings.Contains(rootHTML, `href="../"`) {
		t.Fatal("root should not contain parent link")
	}

	childHTML := renderListing(t, fs, "/child", sortByNameAsc, Listing{ShowPath: true})
	if !strings.Contains(childHTML, `href="../"`) {
		t.Fatal("non-root should contain parent link")
	}
	if !strings.Contains(childHTML, `<tbody><tr><td colspan="2" class="link"><a href="../" title="..">../</a></td>`) {
		t.Fatal("parent link should use fancyindex-style link cell")
	}

	hiddenChildHTML := renderListing(t, fs, "/child", sortByNameAsc, Listing{HideParentDir: true, ShowPath: true})
	if strings.Contains(hiddenChildHTML, `href="../"`) {
		t.Fatal("non-root should not contain parent link when hideParentDir is true")
	}
}

func TestRenderDirectoryListingShowPath(t *testing.T) {
	fs := makeListingFS(t, nil)

	shown := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})
	if !strings.Contains(shown, "<h1>Index of") {
		t.Fatal("expected default title when showPath is true")
	}

	hidden := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: false})
	if strings.Contains(hidden, "<h1>Index of") {
		t.Fatal("expected no title when showPath is false and no custom header is set")
	}

	withHeader := renderListing(t, fs, "/", sortByNameAsc, Listing{
		Header:   "<!DOCTYPE html><html><head></head><body><h1>Custom</h1>",
		Footer:   "</body></html>",
		ShowPath: false,
	})
	if !strings.Contains(withHeader, "<h1>Custom</h1>") {
		t.Fatal("custom header should still render regardless of showPath")
	}
}

func TestRenderDirectoryListingSortByName(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"zebra.txt":  "z",
		"apple.txt":  "a",
		"banana.txt": "b",
	})

	html := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})

	apple := strings.Index(html, "apple.txt")
	banana := strings.Index(html, "banana.txt")
	zebra := strings.Index(html, "zebra.txt")
	require.Less(t, apple, banana, "expected alphabetical order")
	require.Less(t, banana, zebra, "expected alphabetical order")
}

func TestRenderDirectoryListingFancyindexClasses(t *testing.T) {
	fs := makeListingFS(t, map[string]string{
		"file.txt": "content",
	})

	html := renderListing(t, fs, "/", sortByNameAsc, Listing{ShowPath: true})

	if !strings.Contains(html, `<th colspan="2"><a href="?C=N&amp;O=D">File Name ↑</a></th>`) {
		t.Fatal("expected filename header to span two columns")
	}
	if !strings.Contains(html, `<th class="size"><a href="?C=S&amp;O=A">File Size</a></th>`) {
		t.Fatal("expected size header class")
	}
	if !strings.Contains(html, `<th class="date"><a href="?C=M&amp;O=A">Date</a></th>`) {
		t.Fatal("expected date header class")
	}
	if !strings.Contains(html, `<td colspan="2" class="link"><a href="file.txt" title="file.txt">file.txt</a></td>`) {
		t.Fatal("expected file link cell to use fancyindex-style class")
	}
	if !strings.Contains(html, `<td class="size">7 B</td>`) {
		t.Fatal("expected size cell class")
	}
	if !strings.Contains(html, `<td class="date">`) {
		t.Fatal("expected date cell class")
	}
	if strings.Contains(html, `td class="name"`) || strings.Contains(html, `a class="link"`) {
		t.Fatal("expected old listing classes to be removed")
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

func TestSortFileEntriesDirectoriesFirst(t *testing.T) {
	now := time.Now()
	files := []FileEntry{
		{Name: "b-file", IsDir: false, Size: 30, ModTime: now.Add(-3 * time.Hour)},
		{Name: "a-dir", IsDir: true, Size: 0, ModTime: now.Add(-time.Hour)},
		{Name: "c-file", IsDir: false, Size: 10, ModTime: now.Add(-2 * time.Hour)},
		{Name: "z-dir", IsDir: true, Size: 0, ModTime: now.Add(-4 * time.Hour)},
	}

	combos := []ListingSortOptions{
		{Field: listingSortByName, Order: listingSortAsc},
		{Field: listingSortByName, Order: listingSortDesc},
		{Field: listingSortByDate, Order: listingSortAsc},
		{Field: listingSortByDate, Order: listingSortDesc},
		{Field: listingSortBySize, Order: listingSortAsc},
		{Field: listingSortBySize, Order: listingSortDesc},
	}

	for _, sorting := range combos {
		entries := append([]FileEntry(nil), files...)
		sortFileEntries(entries, sorting)

		if !entries[0].IsDir || !entries[1].IsDir {
			t.Fatalf("expected directories first for %+v, got %+v", sorting, entries)
		}
		if entries[2].IsDir || entries[3].IsDir {
			t.Fatalf("expected files after directories for %+v, got %+v", sorting, entries)
		}
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
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}

	for _, tc := range tests {
		if got := formatSize(tc.size); got != tc.expected {
			t.Fatalf("formatSize(%d)=%q want %q", tc.size, got, tc.expected)
		}
	}
}
