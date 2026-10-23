package redisutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRedisConnectionConfiguredCredentialsAndTimeout(t *testing.T) {
	auth := make(chan []string, 1)
	server := newConnectionTestServer(t, func(args []string) string {
		if strings.EqualFold(args[0], "auth") {
			auth <- args
			if !reflect.DeepEqual(args, []string{"auth", "mailer", "secret"}) {
				return "-WRONGPASS invalid credentials\r\n"
			}
		}
		return connectionTestReply(args)
	})
	config := RedisConfig{
		Server: server.listener.Addr().String(), Username: "mailer", Password: "secret", Timeout: 2,
		MinSize: 1, MaxSize: 1, PoolSize: 1,
	}
	original := config
	client, err := NewRedisClient(&config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if config != original {
		t.Fatalf("constructor modified its configuration: got %+v, want %+v", config, original)
	}
	if args := <-auth; !reflect.DeepEqual(args, []string{"auth", "mailer", "secret"}) {
		t.Fatalf("authentication = %v", args)
	}
	options := client.Options()
	if options.Username != "mailer" || options.Password != "secret" {
		t.Fatalf("credentials were not forwarded: username = %q", options.Username)
	}
	if options.DialTimeout != 2*time.Second || options.ReadTimeout != 2*time.Second || options.WriteTimeout != 2*time.Second {
		t.Fatalf("timeouts = %v / %v / %v", options.DialTimeout, options.ReadTimeout, options.WriteTimeout)
	}
}

func TestRedisConnectionDefaultsAndRetries(t *testing.T) {
	server := newConnectionTestServer(t, connectionTestReply)
	for _, test := range []struct {
		name    string
		retries int
		want    int
	}{
		{name: "default", retries: 0, want: 3},
		{name: "disabled", retries: -1, want: 0},
		{name: "configured", retries: 2, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := RedisConfig{Server: server.listener.Addr().String(), MaxRetries: test.retries}
			original := config
			client, err := NewRedisClient(&config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if config != original {
				t.Fatalf("constructor modified configuration: got %+v, want %+v", config, original)
			}
			options := client.Options()
			if options.MaxRetries != test.want {
				t.Fatalf("MaxRetries = %d, want %d", options.MaxRetries, test.want)
			}
			if options.DialTimeout != 5*time.Second || options.ReadTimeout != 5*time.Second || options.WriteTimeout != 5*time.Second {
				t.Fatalf("default timeouts = %v / %v / %v", options.DialTimeout, options.ReadTimeout, options.WriteTimeout)
			}
			if options.ContextTimeoutEnabled || options.MinIdleConns != 0 || options.MaxIdleConns != 0 {
				t.Fatalf("existing context/idle defaults changed: %+v", options)
			}
		})
	}
}

func TestRedisConnectionRejectsInvalidConfig(t *testing.T) {
	for _, test := range []struct {
		name   string
		config *RedisConfig
	}{
		{name: "nil"},
		{name: "negative database", config: &RedisConfig{Db: -1}},
		{name: "negative timeout", config: &RedisConfig{Timeout: -1}},
		{name: "invalid retries", config: &RedisConfig{MaxRetries: -2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewRedisClient(test.config)
			if err == nil || client != nil {
				t.Fatalf("invalid config returned client = %v, error = %v", client, err)
			}
		})
	}
	if strconv.IntSize == 64 {
		overflowing := int64((1<<63-1)/time.Second) + 1
		if client, err := NewRedisClient(&RedisConfig{Timeout: int(overflowing)}); err == nil || client != nil {
			t.Fatalf("overflowing timeout returned client = %v, error = %v", client, err)
		}
	}
}

func TestRedisConnectionContextDeadline(t *testing.T) {
	var pings int
	server := newConnectionTestServer(t, func(args []string) string {
		if strings.EqualFold(args[0], "ping") {
			pings++
			if pings > 1 {
				return ""
			}
		}
		return connectionTestReply(args)
	})
	client, err := NewRedisClient(&RedisConfig{
		Server: server.listener.Addr().String(), Timeout: 2,
		ContextTimeoutEnabled: true, MaxRetries: -1, PoolSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if !client.Options().ContextTimeoutEnabled {
		t.Fatal("ContextTimeoutEnabled was not forwarded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := client.Ping(ctx).Err(); err == nil {
		t.Fatal("unanswered command should time out")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("context deadline ignored: elapsed = %v", elapsed)
	}
}

func TestRedisConnectionStartupTimeoutClosesClient(t *testing.T) {
	server := newConnectionTestServer(t, func(args []string) string {
		if strings.EqualFold(args[0], "ping") {
			return ""
		}
		return connectionTestReply(args)
	})
	start := time.Now()
	client, err := NewRedisClient(&RedisConfig{
		Server: server.listener.Addr().String(), Timeout: 1,
		ContextTimeoutEnabled: true, MaxRetries: -1, MinSize: 2, PoolSize: 3,
	})
	if err == nil || client != nil {
		t.Fatalf("startup returned client = %v, error = %v", client, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("startup did not terminate within its timeout: %v", elapsed)
	}
	server.assertClosed(t)
}

func TestRedisConnectionStartupErrorClosesClient(t *testing.T) {
	server := newConnectionTestServer(t, func(args []string) string {
		if strings.EqualFold(args[0], "ping") {
			return "-ERR startup rejected\r\n"
		}
		return connectionTestReply(args)
	})
	client, err := NewRedisClient(&RedisConfig{
		Server: server.listener.Addr().String(), Timeout: 1, MinSize: 2, PoolSize: 3,
	})
	if err == nil || client != nil {
		t.Fatalf("startup returned client = %v, error = %v", client, err)
	}
	server.assertClosed(t)
}

func TestRedisConnectionPoolForwardsConfig(t *testing.T) {
	server := newConnectionTestServer(t, connectionTestReply)
	config := RedisConfig{
		Server: server.listener.Addr().String(), Timeout: 2, ContextTimeoutEnabled: true, MaxRetries: -1,
	}
	original := config
	pool, err := NewRedisPool(RedisConfigMap{"default": &config})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	client, err := pool.GetRedis()
	if err != nil {
		t.Fatal(err)
	}
	if options := client.Options(); options.ReadTimeout != 2*time.Second || !options.ContextTimeoutEnabled || options.MaxRetries != 0 {
		t.Fatalf("pool did not forward connection options: %+v", options)
	}
	if config != original {
		t.Fatalf("pool modified configuration: got %+v, want %+v", config, original)
	}
}

type connectionTestServer struct {
	listener net.Listener
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

func newConnectionTestServer(t *testing.T, reply func([]string) string) *connectionTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &connectionTestServer{listener: listener, conns: make(map[net.Conn]struct{})}
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			server.mu.Lock()
			server.conns[conn] = struct{}{}
			server.mu.Unlock()
			server.wg.Add(1)
			go func() {
				defer server.wg.Done()
				defer func() {
					_ = conn.Close()
					server.mu.Lock()
					delete(server.conns, conn)
					server.mu.Unlock()
				}()
				reader := bufio.NewReader(conn)
				for {
					args, err := readConnectionTestCommand(reader)
					if err != nil {
						return
					}
					if _, err := io.WriteString(conn, reply(args)); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		server.mu.Lock()
		for conn := range server.conns {
			_ = conn.Close()
		}
		server.mu.Unlock()
		server.wg.Wait()
	})
	return server
}

func (server *connectionTestServer) assertClosed(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		server.mu.Lock()
		active := len(server.conns)
		server.mu.Unlock()
		if active == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("failed constructor leaked %d connections", active)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readConnectionTestCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
	if err != nil || count <= 0 {
		return nil, fmt.Errorf("invalid command header %q", line)
	}
	args := make([]string, count)
	for i := range args {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "$")))
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid argument header %q", line)
		}
		value := make([]byte, size+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, err
		}
		args[i] = string(value[:size])
	}
	return args, nil
}

func connectionTestReply(args []string) string {
	switch strings.ToLower(args[0]) {
	case "hello":
		return "-ERR unknown command 'hello'\r\n"
	case "ping":
		return "+PONG\r\n"
	default:
		return "+OK\r\n"
	}
}
