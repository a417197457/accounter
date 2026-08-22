package observability

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuditRequestsLogsMutationWithoutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	previous := auditLogger
	auditLogger = slog.New(slog.NewJSONHandler(&output, nil))
	defer func() { auditLogger = previous }()

	router := gin.New()
	router.Use(AuditRequests())
	router.POST("/api/transactions", func(c *gin.Context) {
		c.Set("userID", uint(7))
		c.Set("ledgerID", uint(9))
		c.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/transactions", strings.NewReader(`{"remark":"不得进入日志","token":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	logged := output.String()
	for _, expected := range []string{`"event":"business_request"`, `"user_id":7`, `"ledger_id":9`, `"path":"/api/transactions"`, `"success":true`} {
		if !strings.Contains(logged, expected) {
			t.Errorf("audit log missing %s: %s", expected, logged)
		}
	}
	if strings.Contains(logged, "不得进入日志") || strings.Contains(logged, "secret") {
		t.Fatalf("request body leaked into audit log: %s", logged)
	}
}

func TestRotatingWriterRotatesAndCompresses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounter.log")
	w, err := newRotatingWriter(path, 32, 2, 30, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("first log line that fills the file\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("second log line triggers rotation\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active log missing: %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "accounter-*.log.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("compressed backups = %d, want 1", len(backups))
	}
}

func TestAuditRequestsSkipsReadOnlyAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	previous := auditLogger
	auditLogger = slog.New(slog.NewJSONHandler(&output, nil))
	defer func() { auditLogger = previous }()

	router := gin.New()
	router.Use(AuditRequests())
	router.GET("/api/transactions", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/transactions", nil))

	if output.Len() != 0 {
		t.Fatalf("read-only request should not be audited: %s", output.String())
	}
}
