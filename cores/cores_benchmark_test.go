package cores

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
)

func init() {
	logging.Enable(level.LevelError)
}

func simpleHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "默认值"
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK: " + name))
}

func ginHandler(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		name = "默认值"
	}
	c.String(http.StatusOK, "OK: %s", name)
}

func fiberHandler(c *fiber.Ctx) error {
	name := c.Query("name")
	if name == "" {
		name = "默认值"
	}
	return c.SendString("OK: " + name)
}

func BenchmarkNativeHTTP(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", simpleHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(server.URL + "?name=test")
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}

func BenchmarkGin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	gin.DefaultWriter = io.Discard

	router := gin.New()
	router.GET("/", ginHandler)
	server := httptest.NewServer(router)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(server.URL + "?name=test")
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}

func BenchmarkFiber(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
	})
	app.Get("/", fiberHandler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}

	go func() {
		_ = app.Listener(listener)
	}()
	defer app.Shutdown()

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d?name=test", port)

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}

func BenchmarkCoresWithHTTP(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", simpleHandler)

	port := 28080

	url := fmt.Sprintf("http://127.0.0.1:%d?name=test", port)

	srv := NewCores(
		WithHttpHandler("/", mux),
	)

	go func() {
		err := Start(srv, port)
		if err != nil {
			b.Logf("服务启动失败: %v", err)
		}
	}()

	defer func() {
		if srv.cancel != nil {
			srv.cancel()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}

func BenchmarkCoresWithFiber(b *testing.B) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
	})
	app.Get("/", fiberHandler)

	port := 28081

	url := fmt.Sprintf("http://127.0.0.1:%d?name=test", port)

	srv := NewCores(
		WithHttpHandler("/", adaptor.FiberApp(app)),
	)

	go func() {
		err := Start(srv, port)
		if err != nil {
			b.Logf("服务启动失败: %v", err)
		}
	}()

	defer func() {
		if srv.cancel != nil {
			srv.cancel()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}

type SimpleResponse struct {
	Message string `json:"message"`
}

type SimpleHandler struct {
	Name string `query:"name"`
}

func (h SimpleHandler) Handle(c *fiber.Ctx, arg string) (*SimpleResponse, error) {
	return &SimpleResponse{
		Message: arg + ", name: " + h.Name,
	}, nil
}

func helloHandler(c *fiber.Ctx, req *SimpleHandler) (*SimpleResponse, error) {
	return &SimpleResponse{
		Message: "Hello, " + req.Name,
	}, nil
}

func BenchmarkCoresWithGin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
	gin.DefaultWriter = io.Discard

	router := gin.New()
	router.GET("/", ginHandler)

	port := 28082

	url := fmt.Sprintf("http://127.0.0.1:%d?name=test", port)

	srv := NewCores(
		WithHttpHandler("/", router),
	)

	go func() {
		err := Start(srv, port)
		if err != nil {
			b.Logf("服务启动失败: %v", err)
		}
	}()

	defer func() {
		if srv.cancel != nil {
			srv.cancel()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := http.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
}
