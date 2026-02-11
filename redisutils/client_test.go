package redisutils

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

var (
	testRedisClient *RedisClient
)

func TestMain(m *testing.M) {
	testRedisClient = &RedisClient{
		Client: redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
			DB:   0,
		}),
	}
	os.Exit(m.Run())
}

func TestRedisClient_Lock(t *testing.T) {
	ctx := context.Background()
	key := "test_lock"

	// Clean up any existing lock
	testRedisClient.Del(ctx, key)

	// Test successful lock acquisition
	t.Run("acquire lock success", func(t *testing.T) {
		defer cleanupTest(t, testRedisClient, key)
		err := testRedisClient.TryLock(ctx, key, time.Second)
		assert.NoError(t, err)

		// Verify lock exists
		exists, err := testRedisClient.Exists(ctx, key).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), exists)

		// Clean up
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)
	})

	// Test lock conflict
	t.Run("lock conflict", func(t *testing.T) {
		// First lock
		err := testRedisClient.TryLock(ctx, key, time.Second)
		assert.NoError(t, err)

		// Try to acquire same lock
		err = testRedisClient.TryLock(ctx, key, time.Second)
		assert.ErrorIs(t, err, ErrLockAcquireFailed)

		// Clean up
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)
	})

	// Test lock expiration
	t.Run("lock expiration", func(t *testing.T) {
		err := testRedisClient.TryLock(ctx, key, 100*time.Millisecond)
		assert.NoError(t, err)

		// Wait for lock to expire
		time.Sleep(200 * time.Millisecond)

		// Should be able to acquire lock again
		err = testRedisClient.TryLock(ctx, key, time.Second)
		assert.NoError(t, err)

		// Clean up
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)
	})
}

func TestRedisClient_Unlock(t *testing.T) {
	ctx := context.Background()
	key := "test_lock"

	// Clean up any existing lock
	testRedisClient.Del(ctx, key)

	t.Run("unlock success", func(t *testing.T) {
		// First acquire lock
		err := testRedisClient.TryLock(ctx, key, time.Second)
		assert.NoError(t, err)

		// Then unlock
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)

		// Verify lock is gone
		exists, err := testRedisClient.Exists(ctx, key).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), exists)
	})

	t.Run("unlock non-existent lock", func(t *testing.T) {
		err := testRedisClient.Unlock(ctx, "non_existent_lock")
		assert.ErrorIs(t, err, ErrLockNotFound)
	})
}

func TestRedisClient_TryLockWithTimeout(t *testing.T) {
	ctx := context.Background()
	key := "test_lock"

	// Clean up any existing lock
	testRedisClient.Del(ctx, key)

	t.Run("acquire with timeout success", func(t *testing.T) {
		err := testRedisClient.TryLockWithTimeout(ctx, key, time.Second, 500*time.Millisecond)
		assert.NoError(t, err)

		// Clean up
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("acquire with timeout failure", func(t *testing.T) {
		// First lock
		err := testRedisClient.TryLock(ctx, key, time.Second)
		assert.NoError(t, err)

		// Try to acquire with short timeout
		err = testRedisClient.TryLockWithTimeout(ctx, key, time.Second, 200*time.Millisecond)
		assert.ErrorIs(t, err, ErrLockTimeout)

		// Clean up
		err = testRedisClient.Unlock(ctx, key)
		assert.NoError(t, err)
	})
}

func TestRedisClient_ConcurrentLock(t *testing.T) {
	ctx := context.Background()
	key := "test_concurrent_lock"

	// Clean up any existing lock
	testRedisClient.Del(ctx, key)

	t.Run("concurrent lock acquisition", func(t *testing.T) {
		numGoroutines := 10
		successCount := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(numGoroutines)

		// Launch multiple goroutines to acquire lock simultaneously
		for i := 0; i < numGoroutines; i++ {
			go func(routineID int) {
				defer wg.Done()

				err := testRedisClient.TryLock(ctx, key, time.Second)
				if err == nil {
					atomic.AddInt32(&successCount, 1)
					// Simulate some work
					time.Sleep(100 * time.Millisecond)
					// Release the lock
					err = testRedisClient.Unlock(ctx, key)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()
		// Only one goroutine should succeed
		assert.Equal(t, int32(1), successCount)
	})

	t.Run("concurrent lock with timeout", func(t *testing.T) {
		// First acquire the lock to ensure other goroutines will timeout
		err := testRedisClient.TryLock(ctx, key, 2*time.Second)
		assert.NoError(t, err)
		defer testRedisClient.Unlock(ctx, key)

		numGoroutines := 5
		timeout := 500 * time.Millisecond
		successCount := int32(0)
		timeoutCount := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(numGoroutines)

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			go func(routineID int) {
				defer wg.Done()

				err := testRedisClient.TryLockWithTimeout(ctx, key, time.Second, timeout)
				if err == nil {
					atomic.AddInt32(&successCount, 1)
					// Simulate some work
					time.Sleep(50 * time.Millisecond)
					// Release the lock
					err = testRedisClient.Unlock(ctx, key)
					assert.NoError(t, err)
				} else if errors.Is(err, ErrLockTimeout) {
					atomic.AddInt32(&timeoutCount, 1)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		// Verify results
		assert.Equal(t, int32(0), successCount, "No goroutine should acquire the lock")
		assert.Equal(t, int32(numGoroutines), timeoutCount, "All goroutines should timeout")
		assert.True(t, duration >= timeout, "Test should take at least the timeout duration")
	})

	t.Run("concurrent lock and unlock", func(t *testing.T) {
		numIterations := 5
		successCount := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(numIterations)

		// Use channel to control concurrency
		lockChan := make(chan int, numIterations)
		// Initialize tasks
		for i := 0; i < numIterations; i++ {
			lockChan <- i
		}

		// Start multiple workers
		numWorkers := 3
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				for taskID := range lockChan {
					err := testRedisClient.TryLock(ctx, key, time.Second)
					if err == nil {
						atomic.AddInt32(&successCount, 1)
						// Simulate some work
						time.Sleep(10 * time.Millisecond)
						// Release the lock
						err = testRedisClient.Unlock(ctx, key)
						if err != nil {
							t.Logf("Unlock error in worker %d, task %d: %v", workerID, taskID, err)
						}
						wg.Done() // Decrease counter only when lock is acquired successfully
					} else {
						t.Logf("Lock error in worker %d, task %d: %v", workerID, taskID, err)
						// Put the task back to queue if lock acquisition fails
						lockChan <- taskID
					}
					// Small delay between attempts
					time.Sleep(20 * time.Millisecond)
				}
			}(i)
		}

		// Wait for all tasks to complete
		wg.Wait()
		close(lockChan)

		// Verify the total number of completed tasks
		assert.Equal(t, int32(numIterations), successCount,
			"Total successful locks should equal number of iterations")
	})
}

type TestStruct struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Tags    []string `json:"tags"`
	IsAdmin bool     `json:"is_admin"`
}

type ListTestItem struct {
	ID   int      `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type UserProfile struct {
	UserID      int64          `json:"user_id"`
	Username    string         `json:"username"`
	Email       string         `json:"email"`
	Age         int            `json:"age"`
	IsActive    bool           `json:"is_active"`
	Roles       []string       `json:"roles"`
	Preferences map[string]any `json:"preferences"`
	CreatedAt   time.Time      `json:"created_at"`
}

func TestRedisClient_SetValue_GetValue(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		key      string
		value    any
		result   any
		expected any
	}{
		{
			name:     "String Value",
			key:      "test:string",
			value:    "hello world",
			result:   new(string),
			expected: "hello world",
		},
		{
			name:     "Integer Value",
			key:      "test:int",
			value:    42,
			result:   new(int),
			expected: 42,
		},
		{
			name:     "Int64 Value",
			key:      "test:int64",
			value:    int64(9223372036854775807),
			result:   new(int64),
			expected: int64(9223372036854775807),
		},
		{
			name:     "Float32 Value",
			key:      "test:float32",
			value:    float32(3.14),
			result:   new(float32),
			expected: float32(3.14),
		},
		{
			name:     "Float64 Value",
			key:      "test:float64",
			value:    3.14159265359,
			result:   new(float64),
			expected: 3.14159265359,
		},
		{
			name:     "Boolean Value",
			key:      "test:bool",
			value:    true,
			result:   new(bool),
			expected: true,
		},
		{
			name:     "Time Value",
			key:      "test:time",
			value:    time.Date(2024, 3, 14, 15, 9, 26, 0, time.UTC),
			result:   new(time.Time),
			expected: time.Date(2024, 3, 14, 15, 9, 26, 0, time.UTC),
		},
		{
			name:     "Bytes Value",
			key:      "test:bytes",
			value:    []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f},
			result:   new([]byte),
			expected: []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f},
		},
		{
			name: "Struct Value",
			key:  "test:struct",
			value: TestStruct{
				Name:    "John Doe",
				Age:     30,
				Tags:    []string{"admin", "user"},
				IsAdmin: true,
			},
			result: &TestStruct{},
			expected: TestStruct{
				Name:    "John Doe",
				Age:     30,
				Tags:    []string{"admin", "user"},
				IsAdmin: true,
			},
		},
		{
			name: "Map String Interface Value",
			key:  "test:map",
			value: map[string]any{
				"name":    "John",
				"age":     25,
				"scores":  []int{95, 88, 92},
				"active":  true,
				"balance": 123.45,
			},
			result: &map[string]any{},
			expected: map[string]any{
				"name":    "John",
				"age":     float64(25), // JSON numbers are decoded as float64
				"scores":  []any{float64(95), float64(88), float64(92)},
				"active":  true,
				"balance": 123.45,
			},
		},
		{
			name: "Slice Value",
			key:  "test:slice",
			value: []any{
				"string",
				42,
				true,
				3.14,
				[]string{"nested", "slice"},
			},
			result: &[]any{},
			expected: []any{
				"string",
				float64(42),
				true,
				3.14,
				[]any{"nested", "slice"},
			},
		},
		{
			name: "Complex Struct Value",
			key:  "test:complex_struct",
			value: UserProfile{
				UserID:   12345,
				Username: "testuser",
				Email:    "test@example.com",
				Age:      25,
				IsActive: true,
				Roles:    []string{"admin", "user"},
				Preferences: map[string]any{
					"theme":         "dark",
					"language":      "en",
					"timezone":      "UTC",
					"notifications": true,
				},
				CreatedAt: time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC),
			},
			result: &UserProfile{},
			expected: UserProfile{
				UserID:   12345,
				Username: "testuser",
				Email:    "test@example.com",
				Age:      25,
				IsActive: true,
				Roles:    []string{"admin", "user"},
				Preferences: map[string]any{
					"theme":         "dark",
					"language":      "en",
					"timezone":      "UTC",
					"notifications": true,
				},
				CreatedAt: time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test SetValue
			err := testRedisClient.SetValue(ctx, tt.key, tt.value, time.Minute)
			assert.NoError(t, err)

			// Test GetValue
			err = testRedisClient.GetValue(ctx, tt.key, tt.result)
			assert.NoError(t, err)

			// Compare results based on type
			switch v := tt.result.(type) {
			case *string:
				assert.Equal(t, tt.expected, *v)
			case *int:
				assert.Equal(t, tt.expected, *v)
			case *int64:
				assert.Equal(t, tt.expected, *v)
			case *float32:
				assert.InDelta(t, tt.expected, *v, 0.0001)
			case *float64:
				assert.InDelta(t, tt.expected, *v, 0.0000001)
			case *bool:
				assert.Equal(t, tt.expected, *v)
			case *time.Time:
				assert.Equal(t, tt.expected, *v)
			case *[]byte:
				assert.Equal(t, tt.expected, *v)
			case *TestStruct:
				assert.Equal(t, tt.expected, *v)
			case *UserProfile:
				assert.Equal(t, tt.expected, *v)
			case *map[string]any:
				assert.Equal(t, tt.expected, *v)
			case *[]any:
				assert.Equal(t, tt.expected, *v)
			default:
				t.Errorf("未处理的类型: %T", v)
			}

			// Clean up
			testRedisClient.Del(ctx, tt.key)
		})
	}
}

func TestRedisClient_GetValue_TypeConversionErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		value       string
		result      any
		expectedErr string
	}{
		{
			name:        "Invalid Int Conversion",
			value:       "not a number",
			result:      new(int),
			expectedErr: "strconv.Atoi: parsing \"not a number\": invalid syntax",
		},
		{
			name:        "Invalid Float Conversion",
			value:       "not a float",
			result:      new(float64),
			expectedErr: "strconv.ParseFloat: parsing \"not a float\": invalid syntax",
		},
		{
			name:        "Invalid Bool Conversion",
			value:       "not a bool",
			result:      new(bool),
			expectedErr: "strconv.ParseBool: parsing \"not a bool\": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test:conversion"

			// Set the test value
			err := testRedisClient.Set(ctx, key, tt.value, time.Minute).Err()
			assert.NoError(t, err)

			// Try to get with wrong type
			err = testRedisClient.GetValue(ctx, key, tt.result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)

			// Clean up
			testRedisClient.Del(ctx, key)
		})
	}
}

func TestRedisClient_GetValue_NotFound(t *testing.T) {
	ctx := context.Background()

	var result string
	err := testRedisClient.GetValue(ctx, "non:existent:key", &result)
	assert.Error(t, err)
	assert.Equal(t, redis.Nil, err)
}

func TestRedisClient_SetValue_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	// Create a struct with a channel which cannot be JSON marshaled
	invalidStruct := struct {
		Ch chan int
	}{
		Ch: make(chan int),
	}

	err := testRedisClient.SetValue(ctx, "test:invalid", invalidStruct, time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json marshal failed")
}

func TestRedisClient_GetValue_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	// Set invalid JSON string
	err := testRedisClient.Set(ctx, "test:invalid:json", "{invalid json}", time.Minute).Err()
	assert.NoError(t, err)

	var result TestStruct
	err = testRedisClient.GetValue(ctx, "test:invalid:json", &result)
	assert.Error(t, err)

	// Clean up
	testRedisClient.Del(ctx, "test:invalid:json")
}

func TestRedisClient_SetGetValue_Bytes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		value   []byte
		wantErr bool
	}{
		{
			name:    "normal binary data",
			key:     "test_binary",
			value:   []byte{0x00, 0x01, 0x02, 0x03},
			wantErr: false,
		},
		{
			name:    "empty binary data",
			key:     "test_empty_binary",
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "text as binary",
			key:     "test_text_binary",
			value:   []byte("Hello, World!"),
			wantErr: false,
		},
		{
			name:    "binary with null bytes",
			key:     "test_null_binary",
			value:   []byte{0x00, 0xFF, 0x00, 0xFF},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test SetValue
			err := testRedisClient.SetValue(ctx, tt.key, tt.value, time.Minute)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Test GetValue
			var got []byte
			err = testRedisClient.GetValue(ctx, tt.key, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !bytes.Equal(got, tt.value) {
				t.Errorf("GetValue() got = %v, want %v", got, tt.value)
			}
		})
	}
}

func TestRedisClient_SetGetValue_Mixed(t *testing.T) {
	ctx := context.Background()

	t.Run("set string get bytes", func(t *testing.T) {
		key := "test_str_bytes"
		value := "Hello, World!"

		// Set as string
		err := testRedisClient.SetValue(ctx, key, value, time.Minute)
		if err != nil {
			t.Fatalf("SetValue() error = %v", err)
		}

		// Get as bytes
		var got []byte
		err = testRedisClient.GetValue(ctx, key, &got)
		if err != nil {
			t.Fatalf("GetValue() error = %v", err)
		}

		if !bytes.Equal(got, []byte(value)) {
			t.Errorf("GetValue() got = %v, want %v", got, []byte(value))
		}
	})

	t.Run("set bytes get string", func(t *testing.T) {
		key := "test_bytes_str"
		value := []byte("Hello, World!")

		// Set as bytes
		err := testRedisClient.SetValue(ctx, key, value, time.Minute)
		if err != nil {
			t.Fatalf("SetValue() error = %v", err)
		}

		// Get as string
		var got string
		err = testRedisClient.GetValue(ctx, key, &got)
		if err != nil {
			t.Fatalf("GetValue() error = %v", err)
		}

		if got != string(value) {
			t.Errorf("GetValue() got = %v, want %v", got, string(value))
		}
	})
}

func TestRedisClient_ListOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("basic list operations", func(t *testing.T) {
		key := "test:list:basic"
		// Clean up test key
		testRedisClient.Del(ctx, key)

		// Test LPush
		err := testRedisClient.LPushValue(ctx, key, "value1", "value2", "value3")
		assert.NoError(t, err)

		// Test RPush
		err = testRedisClient.RPushValue(ctx, key, "value4", "value5")
		assert.NoError(t, err)

		// Verify list length
		length, err := testRedisClient.LLen(ctx, key).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(5), length)

		// Test LPop
		var leftValue string
		err = testRedisClient.LPopValue(ctx, key, &leftValue)
		assert.NoError(t, err)
		assert.Equal(t, "value3", leftValue)

		// Test RPop
		var rightValue string
		err = testRedisClient.RPopValue(ctx, key, &rightValue)
		assert.NoError(t, err)
		assert.Equal(t, "value5", rightValue)

		// Clean up
		testRedisClient.Del(ctx, key)
	})

	t.Run("complex type list operations", func(t *testing.T) {
		key := "test:list:complex"
		testRedisClient.Del(ctx, key)

		items := []ListTestItem{
			{ID: 1, Name: "Item 1", Tags: []string{"tag1", "tag2"}},
			{ID: 2, Name: "Item 2", Tags: []string{"tag2", "tag3"}},
			{ID: 3, Name: "Item 3", Tags: []string{"tag3", "tag4"}},
		}

		// Test LPush with complex type
		for _, item := range items {
			err := testRedisClient.LPushValue(ctx, key, item)
			assert.NoError(t, err)
		}

		// Test LPop with complex type
		var result ListTestItem
		err := testRedisClient.LPopValue(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, items[len(items)-1], result)

		// Clean up
		testRedisClient.Del(ctx, key)
	})

	t.Run("range operations", func(t *testing.T) {
		key := "test:list:range"
		testRedisClient.Del(ctx, key)

		// Prepare test data
		values := []string{"value1", "value2", "value3", "value4", "value5"}
		for _, v := range values {
			err := testRedisClient.RPushValue(ctx, key, v)
			assert.NoError(t, err)
		}

		// Test Range
		var rangeResult []string
		err := testRedisClient.RangeValue(ctx, key, 1, 3, &rangeResult)
		assert.NoError(t, err)
		assert.Equal(t, []string{"value2", "value3", "value4"}, rangeResult)

		// Test RRange
		var rrangeResult []string
		err = testRedisClient.RRangeValue(ctx, key, 0, 2, &rrangeResult)
		assert.NoError(t, err)
		assert.Equal(t, []string{"value3", "value4", "value5"}, rrangeResult)

		// Clean up
		testRedisClient.Del(ctx, key)
	})

	t.Run("mixed type list operations", func(t *testing.T) {
		key := "test:list:mixed"
		testRedisClient.Del(ctx, key)

		// Create test UserProfile
		userProfile := UserProfile{
			UserID:   12345,
			Username: "test_user",
			Email:    "test@example.com",
			Age:      25,
			IsActive: true,
			Roles:    []string{"admin", "user"},
			Preferences: map[string]any{
				"theme":         "dark",
				"language":      "en",
				"notifications": true,
			},
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		// Note: Since it's LIFO, we need to push in reverse order of desired pop order
		// Test different types of values
		err := testRedisClient.RPushValue(ctx, key, // Use RPush to maintain order
			42,                     // int
			3.14,                   // float64
			true,                   // bool
			"string value",         // string
			map[string]int{"a": 1}, // complex type
			userProfile,            // struct
			[]byte("byte array"),   // []byte - placed at the end
		)
		assert.NoError(t, err)

		// Test popping and converting to correct types
		// Pop in the order of pushing (using LPop)

		// Test int
		var intValue int
		err = testRedisClient.LPopValue(ctx, key, &intValue)
		assert.NoError(t, err)
		assert.Equal(t, 42, intValue)

		// Test float64
		var floatValue float64
		err = testRedisClient.LPopValue(ctx, key, &floatValue)
		assert.NoError(t, err)
		assert.Equal(t, 3.14, floatValue)

		// Test bool
		var boolValue bool
		err = testRedisClient.LPopValue(ctx, key, &boolValue)
		assert.NoError(t, err)
		assert.Equal(t, true, boolValue)

		// Test string
		var strValue string
		err = testRedisClient.LPopValue(ctx, key, &strValue)
		assert.NoError(t, err)
		assert.Equal(t, "string value", strValue)

		// Test map
		var mapValue map[string]int
		err = testRedisClient.LPopValue(ctx, key, &mapValue)
		assert.NoError(t, err)
		assert.Equal(t, map[string]int{"a": 1}, mapValue)

		// Test UserProfile struct
		var profileResult UserProfile
		err = testRedisClient.LPopValue(ctx, key, &profileResult)
		assert.NoError(t, err)
		assert.Equal(t, userProfile, profileResult)

		// Test []byte
		var bytesValue []byte
		err = testRedisClient.LPopValue(ctx, key, &bytesValue)
		assert.NoError(t, err)
		assert.Equal(t, []byte("byte array"), bytesValue)

		// Verify list is empty
		length, err := testRedisClient.LLen(ctx, key).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), length)

		// Clean up
		testRedisClient.Del(ctx, key)
	})

	t.Run("error cases", func(t *testing.T) {
		key := "test:list:errors"
		testRedisClient.Del(ctx, key)

		// Test popping from empty list
		var result string
		err := testRedisClient.LPopValue(ctx, key, &result)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)

		// Test Range parameter validation
		var invalidPtr *string
		err = testRedisClient.RangeValue(ctx, key, 0, 1, invalidPtr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "result must be a slice")

		// Test type conversion error
		err = testRedisClient.LPushValue(ctx, key, "not a number")
		assert.NoError(t, err)

		var intResult int
		err = testRedisClient.LPopValue(ctx, key, &intResult)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `strconv.Atoi: parsing "not a number": invalid syntax`)

		// Clean up
		testRedisClient.Del(ctx, key)
	})
}

func cleanupTest(t *testing.T, client *RedisClient, keys ...string) {
	ctx := context.Background()
	for _, key := range keys {
		err := testRedisClient.Del(ctx, key).Err()
		assert.NoError(t, err)
	}
}

func TestRedisClient_RPushRPop(t *testing.T) {
	ctx := context.Background()
	key := "test:rpush:rpop"
	defer cleanupTest(t, testRedisClient, key)

	// Test basic types
	t.Run("basic types", func(t *testing.T) {
		// Push multiple values
		err := testRedisClient.RPushValue(ctx, key, "value1", 42, true)
		assert.NoError(t, err)

		// Pop and verify
		var strVal string
		err = testRedisClient.RPopValue(ctx, key, &strVal)
		assert.NoError(t, err)
		assert.Equal(t, "1", strVal)

		var intVal int
		err = testRedisClient.RPopValue(ctx, key, &intVal)
		assert.NoError(t, err)
		assert.Equal(t, 42, intVal)

		var lastStr string
		err = testRedisClient.RPopValue(ctx, key, &lastStr)
		assert.NoError(t, err)
		assert.Equal(t, "value1", lastStr)
	})
}

func TestRedisClient_RRangeValue(t *testing.T) {
	ctx := context.Background()
	key := "test:rrange"
	defer cleanupTest(t, testRedisClient, key)

	// Prepare test data
	values := []string{"v1", "v2", "v3", "v4", "v5"}
	for _, v := range values {
		err := testRedisClient.RPushValue(ctx, key, v)
		assert.NoError(t, err)
	}

	t.Run("basic rrange", func(t *testing.T) {
		var result []string
		err := testRedisClient.RRangeValue(ctx, key, 0, 2, &result)
		assert.NoError(t, err)
		assert.Equal(t, []string{"v3", "v4", "v5"}, result)
	})

	t.Run("rrange with invalid indices", func(t *testing.T) {
		// Passing nil pointer should return error
		err := testRedisClient.RRangeValue(ctx, key, 0, 1, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "result must not be nil")
	})
}

func TestRedisClient_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid value conversion", func(t *testing.T) {
		// Test unconvertible type
		ch := make(chan int)
		err := testRedisClient.SetValue(ctx, "test:invalid", ch, time.Minute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "json marshal failed")
	})

	t.Run("invalid pointer types", func(t *testing.T) {
		var invalidPtr *int
		err := testRedisClient.GetValue(ctx, "test:key", invalidPtr)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
	})
}
