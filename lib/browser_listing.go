package lib

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

type ListingSortField string
type ListingSortOrder string

const (
	listingSortByName ListingSortField = "name"
	listingSortByDate ListingSortField = "date"
	listingSortBySize ListingSortField = "size"

	listingSortAsc  ListingSortOrder = "asc"
	listingSortDesc ListingSortOrder = "desc"
)

type ListingSortOptions struct {
	Field ListingSortField
	Order ListingSortOrder
}

// ParseListingSortQuery reads the fancyindex-style ?C=<column>&O=<order> query params.
func ParseListingSortQuery(query url.Values) ListingSortOptions {
	field, ok := listingSortFieldFromCode(query.Get("C"))
	if !ok {
		field = listingSortByName
	}

	order, ok := listingSortOrderFromCode(query.Get("O"))
	if !ok {
		order = listingSortAsc
	}

	return ListingSortOptions{
		Field: field,
		Order: order,
	}
}

type FileEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

func RenderDirectoryListing(ctx context.Context, fs webdav.FileSystem, dirPath string, sorting ListingSortOptions, listing BrowserListing) (string, error) {
	file, err := fs.OpenFile(ctx, dirPath, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open directory: %w", err)
	}
	defer file.Close()

	entries, err := file.Readdir(-1)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	// Convert to FileEntry and sort according to query options.
	var files []FileEntry
	for _, entry := range entries {
		files = append(files, FileEntry{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
		})
	}

	sortFileEntries(files, sorting)

	var buf bytes.Buffer
	collectionPath := normalizeCollectionPath(dirPath)

	buf.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Index of `)
	buf.WriteString(html.EscapeString(collectionPath))
	buf.WriteString(`</title>
<style>
body { font-family: sans-serif; margin: 1rem; }
table { border-collapse: collapse; width: 100%; }
th, td { padding: 0.2rem 0.6rem; text-align: left; white-space: nowrap; }
th.size, td.size { text-align: right; }
td.link { width: 100%; }
</style>
</head>
<body>`)

	if listing.Header != "" {
		buf.WriteString(listing.Header)
	}

	if listing.ShowPath {
		buf.WriteString(`<h1>Index of `)
		buf.WriteString(html.EscapeString(collectionPath))
		buf.WriteString(`</h1>`)
	}

	buf.WriteString(`
<table id="list">
<thead>
<tr>
<th colspan="2"><a href="`)
	buf.WriteString(listingSortLink(listingSortByName, sorting))
	buf.WriteString(`">`)
	buf.WriteString(listingSortLabel("Name", listingSortByName, sorting))
	buf.WriteString(`</a></th>
<th class="size"><a href="`)
	buf.WriteString(listingSortLink(listingSortBySize, sorting))
	buf.WriteString(`">`)
	buf.WriteString(listingSortLabel("Size", listingSortBySize, sorting))
	buf.WriteString(`</a></th>
<th class="date"><a href="`)
	buf.WriteString(listingSortLink(listingSortByDate, sorting))
	buf.WriteString(`">`)
	buf.WriteString(listingSortLabel("Last modified", listingSortByDate, sorting))
	buf.WriteString(`</a></th>
</tr>
</thead>
<tbody>`)

	if dirPath != "/" && !listing.HideParentDir {
		buf.WriteString(`<tr>
<td colspan="2" class="link"><a href="../">../</a></td>
<td class="size">-</td>
<td class="date">-</td>
</tr>
`)
	}

	for _, entry := range files {
		name := html.EscapeString(entry.Name)
		href := fileEntryLink(entry)
		size := "-"
		if entry.IsDir {
			name += "/"
		} else {
			size = formatSize(entry.Size)
		}

		modTime := entry.ModTime.Format("02-Jan-2006 15:04")
		fmt.Fprintf(&buf, `<tr>
<td colspan="2" class="link"><a href="%s">%s</a></td>
<td class="size">%s</td>
<td class="date">%s</td>
</tr>
`, href, name, size, modTime)
	}

	buf.WriteString(`</tbody>
</table>`)

	if listing.Footer != "" {
		buf.WriteString(listing.Footer)
	}

	buf.WriteString(`</body>
</html>`)

	return buf.String(), nil
}

func sortFileEntries(files []FileEntry, sorting ListingSortOptions) {
	sort.SliceStable(files, func(i, j int) bool {
		left := files[i]
		right := files[j]

		less := false
		equal := false
		switch sorting.Field {
		case listingSortByDate:
			less = left.ModTime.Before(right.ModTime)
			equal = left.ModTime.Equal(right.ModTime)
		case listingSortBySize:
			less = left.Size < right.Size
			equal = left.Size == right.Size
		default:
			leftName := strings.ToLower(left.Name)
			rightName := strings.ToLower(right.Name)
			less = leftName < rightName
			equal = leftName == rightName
		}

		if equal {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}

		if sorting.Order == listingSortDesc {
			return !less
		}

		return less
	})
}

func listingSortLink(field ListingSortField, current ListingSortOptions) string {
	order := listingSortAsc
	if current.Field == field {
		if current.Order == listingSortAsc {
			order = listingSortDesc
		} else {
			order = listingSortAsc
		}
	}

	return fmt.Sprintf("?C=%s&O=%s", listingSortFieldToCode(field), listingSortOrderToCode(order))
}

func listingSortLabel(label string, field ListingSortField, current ListingSortOptions) string {
	if current.Field != field {
		return label
	}

	if current.Order == listingSortDesc {
		return label + " ↓"
	}

	return label + " ↑"
}

func normalizeCollectionPath(p string) string {
	clean := path.Clean("/" + strings.TrimSpace(p))
	if clean != "/" {
		clean += "/"
	}
	return clean
}

func fileEntryLink(entry FileEntry) string {
	link := url.PathEscape(filepath.ToSlash(entry.Name))
	if entry.IsDir {
		return link + "/"
	}
	return link
}

// listingSortFieldToCode maps a field to fancyindex's column code (N=name, M=modified, S=size).
func listingSortFieldToCode(field ListingSortField) string {
	switch field {
	case listingSortByDate:
		return "M"
	case listingSortBySize:
		return "S"
	default:
		return "N"
	}
}

func listingSortFieldFromCode(code string) (ListingSortField, bool) {
	switch strings.ToUpper(code) {
	case "N":
		return listingSortByName, true
	case "M":
		return listingSortByDate, true
	case "S":
		return listingSortBySize, true
	default:
		return "", false
	}
}

// listingSortOrderToCode maps an order to fancyindex's code (A=ascending, D=descending).
func listingSortOrderToCode(order ListingSortOrder) string {
	if order == listingSortDesc {
		return "D"
	}
	return "A"
}

func listingSortOrderFromCode(code string) (ListingSortOrder, bool) {
	switch strings.ToUpper(code) {
	case "A":
		return listingSortAsc, true
	case "D":
		return listingSortDesc, true
	default:
		return "", false
	}
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
