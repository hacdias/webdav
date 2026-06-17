package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

const partialUpdateContentType = "application/x-sabredav-partialupdate"

type updateRange struct {
	offset int64
	end    int64
	hasEnd bool
	append bool
}

type partialUpdateError struct {
	status int
	err    error
}

func (e partialUpdateError) Error() string {
	return e.err.Error()
}

func newPartialUpdateError(status int, message string) error {
	return partialUpdateError{status: status, err: errors.New(message)}
}

func writePartialUpdateError(w http.ResponseWriter, err error, fallbackStatus int) {
	var httpErr partialUpdateError
	if errors.As(err, &httpErr) {
		fallbackStatus = httpErr.status
	}
	http.Error(w, err.Error(), fallbackStatus)
}

func (u *handlerUser) handleOptions(w http.ResponseWriter, r *http.Request, reqPath string) {
	allow := "OPTIONS, LOCK, PUT, MKCOL, PATCH"
	if fi, err := u.FileSystem.Stat(r.Context(), reqPath); err == nil {
		if fi.IsDir() {
			allow = "OPTIONS, LOCK, DELETE, PROPPATCH, COPY, MOVE, UNLOCK, PROPFIND"
		} else {
			allow = "OPTIONS, LOCK, GET, HEAD, POST, DELETE, PROPPATCH, COPY, MOVE, UNLOCK, PROPFIND, PUT, PATCH"
		}
	}

	w.Header().Set("Allow", allow)
	w.Header().Set("DAV", "1, 2, sabredav-partialupdate")
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Accept-Patch", partialUpdateContentType)
	w.WriteHeader(http.StatusOK)
}

func (u *handlerUser) handlePartialUpdate(w http.ResponseWriter, r *http.Request, reqPath string) {
	contentRange := r.Header.Get("Content-Range")
	isContentRangePut := r.Method == "PUT" && contentRange != ""

	var (
		updateRange updateRange
		err         error
	)
	if isContentRangePut {
		updateRange, err = parseContentRange(contentRange)
	} else {
		if err := checkPartialUpdateContentType(r.Header.Get("Content-Type")); err != nil {
			http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
			return
		}
		updateRange, err = parseUpdateRange(r.Header.Get("X-Update-Range"))
	}
	if err != nil {
		writePartialUpdateError(w, err, http.StatusBadRequest)
		return
	}
	if r.Method == "PATCH" && r.ContentLength < 0 {
		http.Error(w, "missing content length", http.StatusLengthRequired)
		return
	}

	release, status, err := u.confirmPartialUpdateLocks(r, reqPath)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	defer release()

	ctx := r.Context()
	fi, statErr := u.FileSystem.Stat(ctx, reqPath)
	exists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		http.Error(w, statErr.Error(), http.StatusMethodNotAllowed)
		return
	}
	if exists && fi.IsDir() {
		http.Error(w, "cannot update a collection", http.StatusMethodNotAllowed)
		return
	}

	etag, status, err := u.checkPartialUpdatePreconditions(r, exists, fi)
	if err != nil {
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		http.Error(w, err.Error(), status)
		return
	}

	currentSize := int64(0)
	if exists {
		currentSize = fi.Size()
	}
	if updateRange.append {
		updateRange.offset = currentSize
	} else if updateRange.offset < 0 {
		updateRange.offset += currentSize
		if updateRange.offset < 0 {
			updateRange.offset = 0
		}
	}

	if updateRange.hasEnd {
		expected := updateRange.end - updateRange.offset + 1
		if expected < 0 {
			http.Error(w, "invalid byte range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if r.ContentLength >= 0 && r.ContentLength != expected {
			http.Error(w, "content length does not match byte range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	body := io.Reader(r.Body)
	var cleanup func()
	if updateRange.hasEnd {
		body, cleanup, err = spoolBoundedBody(r.Body, updateRange.end-updateRange.offset+1)
		if err != nil {
			writePartialUpdateError(w, err, http.StatusMethodNotAllowed)
			return
		}
		defer cleanup()
	}

	flag := os.O_RDWR
	if !exists {
		flag |= os.O_CREATE
	}
	f, err := u.FileSystem.OpenFile(ctx, reqPath, flag, 0666)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	if _, err := f.Seek(updateRange.offset, io.SeekStart); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}
	if _, err := io.Copy(f, body); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	if !exists {
		w.WriteHeader(http.StatusCreated)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func checkPartialUpdateContentType(contentType string) error {
	if contentType == "" {
		return errors.New("missing content type")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return err
	}
	if mediaType != partialUpdateContentType {
		return fmt.Errorf("unsupported content type %q", mediaType)
	}
	return nil
}

func (u *handlerUser) checkPartialUpdatePreconditions(r *http.Request, exists bool, fi os.FileInfo) (etag string, status int, err error) {
	ifMatch := r.Header.Get("If-Match")
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifMatch == "" && ifNoneMatch == "" {
		return "", 0, nil
	}

	if ifMatch != "" && !exists {
		return "", http.StatusPreconditionFailed, errors.New("resource does not exist")
	}

	if exists {
		etag, err = findPartialETag(r.Context(), fi)
		if err != nil {
			return "", http.StatusInternalServerError, err
		}
	}

	if ifMatch != "" && !partialETagHeaderMatches(ifMatch, etag, exists) {
		return etag, http.StatusPreconditionFailed, errors.New("etag does not match")
	}

	if ifNoneMatch != "" && exists && partialETagHeaderMatches(ifNoneMatch, etag, true) {
		return etag, http.StatusPreconditionFailed, errors.New("etag matches")
	}

	return etag, 0, nil
}

func findPartialETag(ctx context.Context, fi os.FileInfo) (string, error) {
	if etager, ok := fi.(webdav.ETager); ok {
		etag, err := etager.ETag(ctx)
		if !errors.Is(err, webdav.ErrNotImplemented) {
			return etag, err
		}
	}
	return fmt.Sprintf(`"%x%x"`, fi.ModTime().UnixNano(), fi.Size()), nil
}

func partialETagHeaderMatches(header, etag string, exists bool) bool {
	for _, item := range strings.Split(header, ",") {
		item = strings.TrimSpace(item)
		if item == "*" {
			return exists
		}
		if item == etag || strings.ReplaceAll(item, `\"`, `"`) == etag {
			return true
		}
	}
	return false
}

func parseUpdateRange(header string) (updateRange, error) {
	if header == "" {
		return updateRange{}, errors.New("missing X-Update-Range header")
	}
	if header == "append" {
		return updateRange{append: true}, nil
	}
	if !strings.HasPrefix(header, "bytes=") {
		return updateRange{}, errors.New("invalid X-Update-Range header")
	}
	return parseByteRange(strings.TrimPrefix(header, "bytes="), true)
}

func parseContentRange(header string) (updateRange, error) {
	if !strings.HasPrefix(header, "bytes ") {
		return updateRange{}, errors.New("invalid Content-Range header")
	}
	spec, _, ok := strings.Cut(strings.TrimPrefix(header, "bytes "), "/")
	if !ok {
		return updateRange{}, errors.New("invalid Content-Range header")
	}
	return parseByteRange(spec, false)
}

func parseByteRange(spec string, allowNegativeStart bool) (updateRange, error) {
	if strings.HasPrefix(spec, "-") {
		if !allowNegativeStart {
			return updateRange{}, errors.New("invalid byte range start")
		}
		start, err := strconv.ParseInt(strings.TrimPrefix(spec, "-"), 10, 64)
		if err != nil || start < 0 {
			return updateRange{}, errors.New("invalid byte range start")
		}
		if start == 0 {
			return updateRange{append: true}, nil
		}
		return updateRange{offset: -start}, nil
	}

	startText, endText, ok := strings.Cut(spec, "-")
	if !ok || startText == "" {
		return updateRange{}, errors.New("invalid byte range")
	}

	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil {
		return updateRange{}, errors.New("invalid byte range start")
	}
	if start < 0 && !allowNegativeStart {
		return updateRange{}, errors.New("invalid byte range start")
	}

	r := updateRange{offset: start}
	if endText == "" {
		return r, nil
	}
	if start < 0 {
		return updateRange{}, errors.New("negative byte range cannot include an end")
	}

	end, err := strconv.ParseInt(endText, 10, 64)
	if err != nil {
		return updateRange{}, errors.New("invalid byte range end")
	}
	if end < start {
		return updateRange{}, newPartialUpdateError(http.StatusRequestedRangeNotSatisfiable, "invalid byte range")
	}
	r.end = end
	r.hasEnd = true
	return r, nil
}

func spoolBoundedBody(body io.Reader, expected int64) (io.Reader, func(), error) {
	tmp, err := os.CreateTemp("", "webdav-partial-update-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		name := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(name)
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			cleanup()
		}
	}()

	n, err := io.Copy(tmp, io.LimitReader(body, expected+1))
	if err != nil {
		return nil, nil, err
	}
	if n != expected {
		return nil, nil, newPartialUpdateError(http.StatusRequestedRangeNotSatisfiable, "body length does not match byte range")
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	cleanupOnError = false
	return tmp, cleanup, nil
}

func (u *handlerUser) confirmPartialUpdateLocks(r *http.Request, src string) (release func(), status int, err error) {
	hdr := r.Header.Get("If")
	if hdr == "" {
		now := time.Now()
		token, err := u.LockSystem.Create(now, webdav.LockDetails{
			Root:      src,
			Duration:  -1,
			ZeroDepth: true,
		})
		if err != nil {
			if errors.Is(err, webdav.ErrLocked) {
				return nil, webdav.StatusLocked, err
			}
			return nil, http.StatusInternalServerError, err
		}
		return func() {
			_ = u.LockSystem.Unlock(now, token)
		}, 0, nil
	}

	ifLists, ok := parsePartialIfHeader(hdr)
	if !ok {
		return nil, http.StatusBadRequest, errors.New("webdav: invalid If header")
	}
	for _, l := range ifLists {
		lsrc := l.resourceTag
		if lsrc == "" {
			lsrc = src
		} else {
			parsedURL, err := url.Parse(lsrc)
			if err != nil {
				continue
			}
			if parsedURL.Host != r.Host {
				continue
			}
			lsrc, err = stripPartialPrefix(parsedURL.Path, u.Prefix)
			if err != nil {
				return nil, http.StatusNotFound, err
			}
			if lsrc == "" {
				lsrc = src
			}
		}
		release, err = u.LockSystem.Confirm(time.Now(), lsrc, "", l.conditions...)
		if errors.Is(err, webdav.ErrConfirmationFailed) {
			continue
		}
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return release, 0, nil
	}
	return nil, http.StatusPreconditionFailed, webdav.ErrLocked
}

type partialIfList struct {
	resourceTag string
	conditions  []webdav.Condition
}

func parsePartialIfHeader(header string) ([]partialIfList, bool) {
	s := strings.TrimSpace(header)
	tagged := strings.HasPrefix(s, "<")
	var lists []partialIfList
	for s != "" {
		resourceTag := ""
		if strings.HasPrefix(s, "<") {
			if !tagged {
				return nil, false
			}
			var ok bool
			resourceTag, s, ok = cutPartialIfToken(s, '<', '>')
			if !ok {
				return nil, false
			}
			s = strings.TrimSpace(s)
			if !strings.HasPrefix(s, "(") {
				return nil, false
			}
		}
		for strings.HasPrefix(s, "(") {
			body, rest, ok := cutPartialIfToken(s, '(', ')')
			if !ok {
				return nil, false
			}
			conditions, ok := parsePartialIfConditions(body)
			if !ok {
				return nil, false
			}
			lists = append(lists, partialIfList{resourceTag: resourceTag, conditions: conditions})
			s = strings.TrimSpace(rest)
		}
		if s != "" && !strings.HasPrefix(s, "<") {
			return nil, false
		}
	}
	return lists, len(lists) > 0
}

func parsePartialIfConditions(s string) ([]webdav.Condition, bool) {
	var conditions []webdav.Condition
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return conditions, len(conditions) > 0
		}
		not := false
		if strings.HasPrefix(s, "Not ") || strings.HasPrefix(s, "Not\t") {
			not = true
			s = strings.TrimSpace(s[3:])
		}
		if s == "" {
			return nil, false
		}
		var token string
		switch s[0] {
		case '<':
			var ok bool
			token, s, ok = cutPartialIfToken(s, '<', '>')
			if !ok {
				return nil, false
			}
			conditions = append(conditions, webdav.Condition{Not: not, Token: token})
		case '[':
			var ok bool
			token, s, ok = cutPartialIfToken(s, '[', ']')
			if !ok {
				return nil, false
			}
			conditions = append(conditions, webdav.Condition{Not: not, ETag: token})
		default:
			i := strings.IndexAny(s, " \t")
			if i < 0 {
				token, s = s, ""
			} else {
				token, s = s[:i], s[i:]
			}
			if token == "" || strings.ContainsAny(token, "()<>[]") {
				return nil, false
			}
			conditions = append(conditions, webdav.Condition{Not: not, Token: token})
		}
	}
}

func cutPartialIfToken(s string, open, close byte) (string, string, bool) {
	if s == "" || s[0] != open {
		return "", "", false
	}
	token, rest, ok := strings.Cut(s[1:], string(close))
	return token, rest, ok
}

func stripPartialPrefix(p, prefix string) (string, error) {
	if prefix == "" {
		return p, nil
	}
	if stripped := strings.TrimPrefix(p, prefix); len(stripped) < len(p) {
		return stripped, nil
	}
	return "", errors.New("webdav: prefix mismatch")
}
