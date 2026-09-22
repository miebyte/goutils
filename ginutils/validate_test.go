package ginutils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/miebyte/goutils/structutils"
)

func TestValidateRequestDataKeepsContext(t *testing.T) {
	type contextKey struct{}
	type request struct {
		Name string `validate:"ginutils_request_context"`
	}

	err := structutils.Validator().RegisterValidationCtx("ginutils_request_context", func(ctx context.Context, field validator.FieldLevel) bool {
		return ctx.Value(contextKey{}) == "tenant-1"
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), contextKey{}, "tenant-1")

	t.Run("struct", func(t *testing.T) {
		req := request{Name: "alice"}
		if err := validateRequestData(ctx, &req, reflect.Struct); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("slice", func(t *testing.T) {
		req := []request{{Name: "alice"}}
		if err := validateRequestData(ctx, &req, reflect.Slice); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ShouldBind", func(t *testing.T) {
		c, _ := newTestContext()
		c.Request = httptest.NewRequest(http.MethodGet, "/?Name=alice", nil).WithContext(ctx)
		var req request
		if err := ShouldBind(c, &req); err != nil {
			t.Fatal(err)
		}
	})
}
