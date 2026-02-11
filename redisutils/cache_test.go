package redisutils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheContext_DoFunc_CacheHit(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	fn := func(ctx context.Context, id int) (string, error) {
		atomic.AddInt32(&callCount, 1)
		return fmt.Sprintf("val:%d", id), nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:do").
		SetTTL(time.Minute).
		SetTTLJitter(0)
		// SetArgsKeyFunc(func(funcName string, arg int) (string, error) {
		// 	return "testkeyfunc", nil
		// })
	wrapped := cc.Func()

	key, err := cc.buildRedisKey(ctx, 1)
	assert.NoError(t, err)
	t.Logf("key: %s", key)
	defer cleanupTest(t, testRedisClient, key)

	got1, err := wrapped(ctx, 1)
	assert.NoError(t, err)
	got2, err := wrapped(ctx, 1)
	assert.NoError(t, err)

	assert.Equal(t, "val:1", got1)
	assert.Equal(t, got1, got2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCacheContext_OnceFunc_Singleflight(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	fn := func(ctx context.Context, id int) (int, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		return id * 2, nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:once").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.OnceFunc()

	key, err := cc.buildRedisKey(ctx, 2)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	const goroutines = 10
	results := make(chan int, goroutines)
	errs := make(chan error, goroutines)
	wg := sync.WaitGroup{}
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			val, err := wrapped(ctx, 2)
			results <- val
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	for val := range results {
		assert.Equal(t, 4, val)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCacheContext_DoFunc_NoCacheOnError(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	fn := func(ctx context.Context, id int) (string, error) {
		atomic.AddInt32(&callCount, 1)
		return "", errors.New("boom")
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:error").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.Func()

	key, err := cc.buildRedisKey(ctx, 3)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	got1, err := wrapped(ctx, 3)
	t.Logf("err: %v", err)
	assert.Error(t, err)
	got2, err := wrapped(ctx, 3)
	assert.Error(t, err)

	assert.Equal(t, "", got1)
	assert.Equal(t, "", got2)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))

	exists, err := testRedisClient.Exists(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

func TestCacheContext_CacheTTL_JitterRange(t *testing.T) {
	cc := &CacheContext[int, int]{
		ttl:       time.Minute,
		ttlJitter: time.Second * 30,
	}

	for range 10 {
		ttl := cc.cacheTTL()
		t.Logf("ttl: %s", ttl)
		assert.GreaterOrEqual(t, ttl, cc.ttl)
		assert.LessOrEqual(t, ttl, cc.ttl+cc.ttlJitter)
	}
}

func TestCacheContext_Func_MultiReturnTypes(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	type profile struct {
		ID   int
		Name string
	}

	labels := []string{"a", "b", "a"}
	type req struct {
		ID   int
		Tags []string
	}
	type result struct {
		Profile profile
		Stats   map[string]int
	}
	fn := func(ctx context.Context, input req) (result, error) {
		atomic.AddInt32(&callCount, 1)
		counts := make(map[string]int, len(input.Tags))
		for _, tag := range input.Tags {
			counts[tag]++
		}
		return result{
			Profile: profile{ID: input.ID, Name: fmt.Sprintf("user:%d", input.ID)},
			Stats:   counts,
		}, nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:multi").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.Func()

	input := req{ID: 7, Tags: labels}
	key, err := cc.buildRedisKey(ctx, input)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	got1, err := wrapped(ctx, input)
	assert.NoError(t, err)
	got2, err := wrapped(ctx, input)
	assert.NoError(t, err)

	assert.Equal(t, result{
		Profile: profile{ID: 7, Name: "user:7"},
		Stats:   map[string]int{"a": 2, "b": 1},
	}, got1)
	assert.Equal(t, got1, got2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCacheContext_Func_BytesAndSliceReturn(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	type req struct {
		Payload []byte
		Items   []int
	}
	type result struct {
		Payload []byte
		Items   []int
	}
	fn := func(ctx context.Context, input req) (result, error) {
		atomic.AddInt32(&callCount, 1)
		outPayload := append([]byte(nil), input.Payload...)
		outItems := append([]int(nil), input.Items...)
		return result{Payload: outPayload, Items: outItems}, nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:bytes").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.Func()

	payload := []byte("hello")
	items := []int{1, 2, 3}
	input := req{Payload: payload, Items: items}
	key, err := cc.buildRedisKey(ctx, input)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	res1, err := wrapped(ctx, input)
	assert.NoError(t, err)
	res2, err := wrapped(ctx, input)
	assert.NoError(t, err)

	assert.Equal(t, payload, res1.Payload)
	assert.Equal(t, items, res1.Items)
	assert.Equal(t, res1, res2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCacheContext_Func_StructArgsAndReturn(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	type Address struct {
		City string
		Zip  int
	}
	type User struct {
		ID      int
		Name    string
		Active  bool
		Score   float64
		Created time.Time
		Tags    []string
		Addr    Address
	}
	type Result struct {
		UserID int
		Ok     bool
		Meta   map[string]string
	}

	createdAt := time.Now().Round(0)
	type req struct {
		User User
		Opts map[string]int
	}
	type result struct {
		Res  Result
		User *User
	}
	fn := func(ctx context.Context, input req) (result, error) {
		atomic.AddInt32(&callCount, 1)
		meta := map[string]string{
			"name":  input.User.Name,
			"level": fmt.Sprintf("%d", input.Opts["level"]),
		}
		outUser := input.User
		return result{
			Res:  Result{UserID: input.User.ID, Ok: input.User.Active, Meta: meta},
			User: &outUser,
		}, nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:struct").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.Func()

	user := User{
		ID:      101,
		Name:    "alice",
		Active:  true,
		Score:   9.5,
		Created: createdAt,
		Tags:    []string{"x", "y"},
		Addr:    Address{City: "sh", Zip: 200000},
	}
	opts := map[string]int{"level": 3}
	input := req{User: user, Opts: opts}
	key, err := cc.buildRedisKey(ctx, input)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	got1, err := wrapped(ctx, input)
	assert.NoError(t, err)
	got2, err := wrapped(ctx, input)
	assert.NoError(t, err)

	assert.Equal(t, result{
		Res: Result{
			UserID: 101,
			Ok:     true,
			Meta: map[string]string{
				"name":  "alice",
				"level": "3",
			},
		},
		User: &user,
	}, got1)
	assert.Equal(t, got1, got2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCacheContext_Func_StructSliceArgs(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	type Item struct {
		Code  string
		Count int
		Price float64
	}
	type Summary struct {
		TotalCount int
		TotalPrice float64
		Keys       []string
	}

	type req struct {
		Items []Item
		Flags map[string]bool
	}
	fn := func(ctx context.Context, input req) (Summary, error) {
		atomic.AddInt32(&callCount, 1)
		var totalCount int
		var totalPrice float64
		keys := make([]string, 0, len(input.Items))
		for _, it := range input.Items {
			totalCount += it.Count
			totalPrice += it.Price * float64(it.Count)
			keys = append(keys, it.Code)
		}
		if input.Flags["reverse"] {
			for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
		return Summary{TotalCount: totalCount, TotalPrice: totalPrice, Keys: keys}, nil
	}

	cc := CacheCall(ctx, fn).
		WithClient(testRedisClient).
		SetPrefix("test:cache:struct-slice").
		SetTTL(time.Minute).
		SetTTLJitter(0)
	wrapped := cc.Func()

	items := []Item{
		{Code: "a", Count: 2, Price: 1.5},
		{Code: "b", Count: 1, Price: 4.0},
	}
	flags := map[string]bool{"reverse": true}
	input := req{Items: items, Flags: flags}
	key, err := cc.buildRedisKey(ctx, input)
	assert.NoError(t, err)
	defer cleanupTest(t, testRedisClient, key)

	sum1, err := wrapped(ctx, input)
	assert.NoError(t, err)
	sum2, err := wrapped(ctx, input)
	assert.NoError(t, err)

	assert.Equal(t, Summary{
		TotalCount: 3,
		TotalPrice: 7.0,
		Keys:       []string{"b", "a"},
	}, sum1)
	assert.Equal(t, sum1, sum2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}
