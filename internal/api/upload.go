package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/rainchan/webmux/internal/session"
)

const (
	maxUploadBytes     int64 = 100 << 20
	maxUploadBodyBytes       = maxUploadBytes + 1<<20
	uploadReadTimeout        = 5 * time.Minute
)

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	item := s.find(w, r)
	if item == nil {
		return
	}
	if item.Info().Status == session.StatusExited {
		failure(w, http.StatusConflict, errors.New("cannot upload to an exited session"))
		return
	}

	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(uploadReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		failure(w, http.StatusInternalServerError, fmt.Errorf("set upload read deadline: %w", err))
		return
	}
	// Interrupt a blocked network read on cancellation, and restore the deadline
	// before returning so this route's longer deadline cannot affect other routes.
	cancelled := make(chan struct{})
	stop := context.AfterFunc(r.Context(), func() {
		_ = controller.SetReadDeadline(time.Now())
		close(cancelled)
	})
	defer func() {
		if !stop() {
			<-cancelled
		}
		_ = controller.SetReadDeadline(time.Time{})
	}()

	if r.ContentLength > maxUploadBodyBytes {
		failure(w, http.StatusRequestEntityTooLarge, errors.New("upload request exceeds 101 MiB"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		failure(w, http.StatusBadRequest, fmt.Errorf("read upload multipart body: %w", err))
		return
	}
	part, err := reader.NextRawPart()
	if err != nil {
		uploadReadFailure(w, "expected exactly one file part named file", err)
		return
	}
	disposition, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil || disposition != "form-data" || params["name"] != "file" || params["filename"] == "" {
		failure(w, http.StatusBadRequest, errors.New("expected exactly one file part named file with a filename"))
		return
	}
	name, err := uploadFilename(params["filename"])
	if err != nil {
		failure(w, http.StatusBadRequest, err)
		return
	}

	dir, err := os.MkdirTemp("", "webmux-upload-")
	if err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("create upload temporary directory: %w", err))
		return
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.Chmod(dir, 0700); err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("set upload directory permissions: %w", err))
		return
	}
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("resolve upload temporary directory: %w", err))
		return
	}
	path := filepath.Join(absoluteDir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("create uploaded file: %w", err))
		return
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("set uploaded file permissions: %w", err))
		return
	}
	written, err := copyUpload(file, part, r.Context(), maxUploadBytes)
	if err != nil {
		var pathError *os.PathError
		if errors.As(err, &pathError) && pathError.Op == "write" {
			failure(w, http.StatusInternalServerError, fmt.Errorf("write uploaded file: %w", err))
		} else {
			uploadReadFailure(w, "read uploaded file", err)
		}
		return
	}
	if written > maxUploadBytes {
		failure(w, http.StatusRequestEntityTooLarge, errors.New("uploaded file exceeds 100 MiB"))
		return
	}
	if err := file.Close(); err != nil {
		failure(w, http.StatusInternalServerError, fmt.Errorf("close uploaded file: %w", err))
		return
	}
	if _, err := reader.NextRawPart(); err != io.EOF {
		if err == nil {
			failure(w, http.StatusBadRequest, errors.New("expected exactly one file part named file; extra multipart part received"))
		} else {
			uploadReadFailure(w, "finish upload multipart body", err)
		}
		return
	}
	// Multipart permits an epilogue. Drain it through the bounded body reader so
	// even chunked requests cannot bypass the total request size limit.
	if _, err := io.Copy(io.Discard, uploadContextReader{r.Context(), r.Body}); err != nil {
		uploadReadFailure(w, "finish upload request", err)
		return
	}
	if err := r.Context().Err(); err != nil {
		uploadReadFailure(w, "upload cancelled", err)
		return
	}
	if item.Info().Status == session.StatusExited {
		failure(w, http.StatusConflict, errors.New("session exited during upload"))
		return
	}
	complete = true
	reply(w, http.StatusCreated, map[string]string{"path": path})
}

func uploadFilename(name string) (string, error) {
	for _, ch := range name {
		if unicode.IsControl(ch) {
			return "", errors.New("upload filename contains an unsafe control character")
		}
	}
	// Treat both slash styles as separators, including filenames supplied by
	// Windows clients. Inspect the original name before discarding directories.
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return "", errors.New("upload filename must name a file")
	}
	return name, nil
}

type uploadContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r uploadContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if cancelled := r.ctx.Err(); cancelled != nil {
		return n, cancelled
	}
	return n, err
}

// Read at most one byte beyond the file limit to distinguish a file exactly at
// the limit from an oversized file without buffering the upload in memory.
func copyUpload(dst io.Writer, src io.Reader, ctx context.Context, limit int64) (int64, error) {
	return io.Copy(dst, io.LimitReader(uploadContextReader{ctx, src}, limit+1))
}

func uploadReadFailure(w http.ResponseWriter, operation string, err error) {
	status := http.StatusBadRequest
	var tooLarge *http.MaxBytesError
	var networkError net.Error
	if errors.As(err, &tooLarge) {
		status = http.StatusRequestEntityTooLarge
	} else if errors.As(err, &networkError) && networkError.Timeout() {
		status = http.StatusRequestTimeout
	}
	failure(w, status, fmt.Errorf("%s: %w", operation, err))
}
