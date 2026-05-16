package lib

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const partialUpdateContentType = "application/x-sabredav-partialupdate"

type updateRange struct {
	offset int64
	end    int64
	hasEnd bool
	append bool
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	if r.Header.Get("If-None-Match") == "*" && exists {
		http.Error(w, "resource already exists", http.StatusPreconditionFailed)
		return
	}
	if r.Header.Get("If-Match") == "*" && !exists {
		http.Error(w, "resource does not exist", http.StatusPreconditionFailed)
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
			http.Error(w, "invalid byte range", http.StatusBadRequest)
			return
		}
		if r.ContentLength >= 0 && r.ContentLength != expected {
			http.Error(w, "content length does not match byte range", http.StatusBadRequest)
			return
		}
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
	n, err := io.Copy(f, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}
	if updateRange.hasEnd && n != updateRange.end-updateRange.offset+1 {
		http.Error(w, "body length does not match byte range", http.StatusBadRequest)
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
		return updateRange{}, errors.New("invalid byte range")
	}
	r.end = end
	r.hasEnd = true
	return r, nil
}
