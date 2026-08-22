package handler

import (
	"net/http/httptest"
	"testing"

	"accounter/backend/internal/service"

	"github.com/gin-gonic/gin/binding"
)

func TestListQueryAllowsPlainDateForManualParsing(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/transactions?start=2026-08-03&end=2026-08-03", nil)
	var params service.ListRequest
	if err := binding.Query.Bind(req, &params); err != nil {
		t.Fatalf("YYYY-MM-DD 不应在自动绑定阶段报错: %v", err)
	}
	if params.Start != nil || params.End != nil {
		t.Fatal("start/end 应保留给 handler 的 parseTime 手动解析")
	}
}

func TestListQueryAllowsDefaultPagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/transactions", nil)
	var params service.ListRequest
	if err := binding.Query.Bind(req, &params); err != nil {
		t.Fatalf("省略分页参数时应由 handler 填默认值: %v", err)
	}
}
