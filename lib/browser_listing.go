package lib

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

//go:embed browser_listing.html
var listingTemplateSource string

var listingTemplate = template.Must(template.New("listing").Parse(listingTemplateSource))

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
// Defaults to sorting by date, descending, when no query params are given.
func ParseListingSortQuery(query url.Values) ListingSortOptions {
	field, ok := listingSortFieldFromCode(query.Get("C"))
	if !ok {
		field = listingSortByDate
	}

	order, ok := listingSortOrderFromCode(query.Get("O"))
	if !ok {
		order = listingSortDesc
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

// listingPage is the data handed to browser_listing.html.
type listingPage struct {
	Path       string
	ShowPath   bool
	ShowParent bool
	// Header and Footer come from the operator's configuration and are
	// therefore trusted: they are the only values rendered without escaping.
	Header     template.HTML
	Footer     template.HTML
	NameColumn listingColumn
	SizeColumn listingColumn
	DateColumn listingColumn
	Entries    []listingEntry
}

type listingColumn struct {
	Href  string
	Label string
}

type listingEntry struct {
	Name    string
	Href    string
	IsDir   bool
	Size    string
	ModTime string
}

func RenderDirectoryListing(ctx context.Context, fs webdav.FileSystem, dirPath string, sorting ListingSortOptions, listing BrowserListing) (string, error) {
	file, err := fs.OpenFile(ctx, dirPath, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open directory: %w", err)
	}
	defer func() { _ = file.Close() }()

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

	page := listingPage{
		Path:       normalizeCollectionPath(dirPath),
		ShowPath:   listing.ShowPath,
		ShowParent: dirPath != "/" && !listing.HideParentDir,
		Header:     template.HTML(listing.Header),
		Footer:     template.HTML(listing.Footer),
		NameColumn: listingSortColumn("File Name", listingSortByName, sorting),
		SizeColumn: listingSortColumn("File Size", listingSortBySize, sorting),
		DateColumn: listingSortColumn("Date", listingSortByDate, sorting),
	}

	for _, entry := range files {
		size := "-"
		if !entry.IsDir {
			size = formatSize(entry.Size)
		}

		page.Entries = append(page.Entries, listingEntry{
			Name:    entry.Name,
			Href:    fileEntryLink(entry),
			IsDir:   entry.IsDir,
			Size:    size,
			ModTime: entry.ModTime.Format("02-Jan-2006 15:04"),
		})
	}

	var buf bytes.Buffer
	if err := listingTemplate.Execute(&buf, page); err != nil {
		return "", fmt.Errorf("failed to render directory listing: %w", err)
	}

	return buf.String(), nil
}

func sortFileEntries(files []FileEntry, sorting ListingSortOptions) {
	sort.SliceStable(files, func(i, j int) bool {
		left := files[i]
		right := files[j]

		// Directories always come before files, regardless of sort order.
		if left.IsDir != right.IsDir {
			return left.IsDir
		}

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

func listingSortColumn(label string, field ListingSortField, current ListingSortOptions) listingColumn {
	return listingColumn{
		Href:  listingSortLink(field, current),
		Label: listingSortLabel(label, field, current),
	}
}

func normalizeCollectionPath(p string) string {
	clean := path.Clean("/" + strings.TrimSpace(p))
	if clean != "/" {
		clean += "/"
	}
	return clean
}

// fileEntryLink builds a relative link to an entry. url.URL.String escapes the
// name and prefixes it with "./" when it contains a colon, so that a name such
// as "javascript:alert(1)" can never be read as a URL scheme.
func fileEntryLink(entry FileEntry) string {
	link := (&url.URL{Path: entry.Name}).String()
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

// formatSize mimics nginx-fancyindex's IEC binary units (e.g. "13.8 MiB").
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
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
