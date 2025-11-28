package ginutils

import (
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/miebyte/goutils/structutils"
)

type bindStrategy interface {
	Need(c *gin.Context) bool
	Bind(c *gin.Context, obj any) error
}

type headerBind struct{}

func (b *headerBind) Need(c *gin.Context) bool {
	return len(c.Request.Header) > 0
}

func (b *headerBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindHeader(obj)
}

type urlBind struct{}

func (b *urlBind) Need(c *gin.Context) bool {
	return len(c.Params) > 0
}

func (b *urlBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindUri(obj)
}

type queryBind struct{}

func (b *queryBind) Need(c *gin.Context) bool {
	return len(c.Request.URL.Query()) > 0
}

func (b *queryBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindQuery(obj)
}

type bodyBind struct{}

func (b *bodyBind) Need(c *gin.Context) bool {
	return c.Request.ContentLength > 0
}

func (b *bodyBind) Bind(c *gin.Context, obj any) error {
	binder := binding.Default(c.Request.Method, c.ContentType())
	return c.ShouldBindWith(obj, binder)
}

func ShouldBind(c *gin.Context, obj any) error {
	err := c.ShouldBind(obj)
	if err != nil {
		return err
	}

	return structutils.Validator().Struct(obj)
}

var (
	strategies = []bindStrategy{
		&bodyBind{},
		&headerBind{},
		&urlBind{},
		&queryBind{},
	}
)

func bindRequestData(c *gin.Context, reqPtr any, reqStrategies []bindStrategy) error {
	for _, strategy := range reqStrategies {
		if strategy.Need(c) {
			if err := strategy.Bind(c, reqPtr); err != nil {
				return err
			}
		}
	}

	return nil
}

func resolveStrategies(t reflect.Type) []bindStrategy {
	base := t
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	if base.Kind() == reflect.Slice || base.Kind() == reflect.Array {
		return []bindStrategy{&bodyBind{}}
	}

	var (
		hasBody   bool
		hasPath   bool
		hasQuery  bool
		hasHeader bool
	)

	var traverse func(t reflect.Type)
	traverse = func(t reflect.Type) {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Anonymous {
				traverse(field.Type)
				continue
			}

			// skip unexported fields
			if field.PkgPath != "" {
				continue
			}

			tag := field.Tag

			isUri := tag.Get("uri") != ""
			isForm := tag.Get("form") != ""
			isHeader := tag.Get("header") != ""
			isBody := tag.Get("json") != "" || tag.Get("xml") != "" || tag.Get("yaml") != "" || tag.Get("protobuf") != ""

			if isUri {
				hasPath = true
			}
			if isForm {
				hasQuery = true
				hasBody = true
			}
			if isHeader {
				hasHeader = true
			}
			if isBody {
				hasBody = true
			}

			// If no strategy tag is present, assume body (default JSON behavior)
			if !isUri && !isForm && !isHeader && !isBody {
				hasBody = true
			}
		}
	}

	traverse(t)

	var final []bindStrategy
	for _, s := range strategies {
		switch s.(type) {
		case *bodyBind:
			if hasBody {
				final = append(final, s)
			}
		case *urlBind:
			if hasPath {
				final = append(final, s)
			}
		case *queryBind:
			if hasQuery {
				final = append(final, s)
			}
		case *headerBind:
			if hasHeader {
				final = append(final, s)
			}
		}
	}

	return final
}
