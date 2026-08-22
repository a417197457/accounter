package observability

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "requestID"

var auditLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Setup configures process logs and audit logs. Production logs are written to
// both stdout/journald and a size-rotated file.
func Setup(file string, maxSizeMB, maxBackups, maxAgeDays int, compress bool) (io.Closer, error) {
	if file == "" {
		log.SetOutput(os.Stdout)
		gin.DefaultWriter = os.Stdout
		gin.DefaultErrorWriter = os.Stderr
		auditLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return nil, fmt.Errorf("创建日志目录: %w", err)
	}
	rotator, err := newRotatingWriter(file, int64(maxSizeMB)*1024*1024, maxBackups, maxAgeDays, compress)
	if err != nil {
		return nil, err
	}
	writer := io.MultiWriter(os.Stdout, rotator)
	log.SetOutput(writer)
	gin.DefaultWriter = writer
	gin.DefaultErrorWriter = writer
	auditLogger = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return rotator, nil
}

type rotatingWriter struct {
	mu         sync.Mutex
	filename   string
	maxBytes   int64
	maxBackups int
	maxAge     time.Duration
	compress   bool
	file       *os.File
	size       int64
}

func newRotatingWriter(filename string, maxBytes int64, maxBackups, maxAgeDays int, compress bool) (*rotatingWriter, error) {
	if maxBytes <= 0 {
		maxBytes = 20 * 1024 * 1024
	}
	w := &rotatingWriter{
		filename:   filename,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
		maxAge:     time.Duration(maxAgeDays) * 24 * time.Hour,
		compress:   compress,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	if err := w.cleanup(); err != nil {
		_ = w.Close()
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("打开日志文件: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("读取日志文件状态: %w", err)
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil

	ext := filepath.Ext(w.filename)
	base := strings.TrimSuffix(w.filename, ext)
	backup := fmt.Sprintf("%s-%s%s", base, time.Now().Format("20060102T150405.000000000"), ext)
	if err := os.Rename(w.filename, backup); err != nil {
		return fmt.Errorf("轮转日志文件: %w", err)
	}
	if w.compress {
		if err := gzipFile(backup); err != nil {
			return err
		}
	}
	if err := w.open(); err != nil {
		return err
	}
	return w.cleanup()
}

func (w *rotatingWriter) cleanup() error {
	ext := filepath.Ext(w.filename)
	base := strings.TrimSuffix(w.filename, ext)
	matches, err := filepath.Glob(base + "-*" + ext + "*")
	if err != nil {
		return fmt.Errorf("查找轮转日志: %w", err)
	}
	type backupFile struct {
		path    string
		modTime time.Time
	}
	backups := make([]backupFile, 0, len(matches))
	for _, path := range matches {
		info, statErr := os.Stat(path)
		if statErr == nil && info.Mode().IsRegular() {
			backups = append(backups, backupFile{path: path, modTime: info.ModTime()})
		}
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].modTime.After(backups[j].modTime) })
	now := time.Now()
	for index, backup := range backups {
		expired := w.maxAge > 0 && now.Sub(backup.modTime) > w.maxAge
		excess := w.maxBackups >= 0 && index >= w.maxBackups
		if expired || excess {
			if err := os.Remove(backup.path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("清理旧日志: %w", err)
			}
		}
	}
	return nil
}

func gzipFile(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开待压缩日志: %w", err)
	}
	destination, err := os.OpenFile(path+".gz", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("创建压缩日志: %w", err)
	}
	zipper := gzip.NewWriter(destination)
	_, copyErr := io.Copy(zipper, source)
	closeZipErr := zipper.Close()
	closeDestErr := destination.Close()
	closeSourceErr := source.Close()
	if copyErr != nil || closeZipErr != nil || closeDestErr != nil || closeSourceErr != nil {
		_ = os.Remove(path + ".gz")
		return fmt.Errorf("压缩轮转日志失败")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("删除未压缩日志: %w", err)
	}
	return nil
}

// AuditRequests records mutating API calls without request bodies, tokens,
// remarks, chat messages, or audio content.
func AuditRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Set(requestIDKey, requestID)
		c.Header("X-Request-ID", requestID)

		started := time.Now()
		c.Next()

		if !isAuditedRequest(c.Request.Method, c.Request.URL.Path) {
			return
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		Event(c, "business_request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"success", c.Writer.Status() < 400,
			"latency_ms", time.Since(started).Milliseconds(),
		)
	}
}

// Event writes a structured business audit event with request context.
func Event(c *gin.Context, event string, attrs ...any) {
	common := []any{
		"log_type", "audit",
		"event", event,
		"request_id", RequestID(c),
		"user_id", contextUint(c, "userID"),
		"ledger_id", contextUint(c, "ledgerID"),
		"client_ip", c.ClientIP(),
	}
	auditLogger.Info("business_audit", append(common, attrs...)...)
}

func RequestID(c *gin.Context) string {
	value, _ := c.Get(requestIDKey)
	requestID, _ := value.(string)
	return requestID
}

func contextUint(c *gin.Context, key string) uint {
	value, ok := c.Get(key)
	if !ok {
		return 0
	}
	n, _ := value.(uint)
	return n
}

func isAuditedRequest(method, path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	case "GET":
		return path == "/api/report/export"
	default:
		return false
	}
}

func newRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}
