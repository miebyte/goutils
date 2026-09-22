package ginutils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type mountTestRequest struct {
	Name string `json:"name" validate:"required"`
}

func (r *mountTestRequest) Handle(c *gin.Context) (*string, error) {
	return &r.Name, nil
}

// TestMountHandlerValidRequest 验证合法请求能够完成绑定、清洗并调用 Handle。
// 当前实现会把 **mountTestRequest 传给 mold，预期本用例在修复前失败。
func TestMountHandlerValidRequest(t *testing.T) {
	c, w := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	MountHandler[*mountTestRequest, string]()(c)

	ret := decodeRet(t, w)
	if ret.Code != 0 || ret.Data != "alice" {
		t.Fatalf("response = %+v, want code=0 data=alice", ret)
	}
}
