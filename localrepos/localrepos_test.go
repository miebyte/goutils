package localrepos

import (
	"context"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testData = []TestData{
		{id: "1", value: "test1"},
		{id: "2", value: "test2"},
		{id: "3", value: "test3"},
	}
)

// 测试用的数据结构
type TestData struct {
	id    string
	value string
}

func (t TestData) GetKey() string {
	return t.id
}

func mockDataStore(ctx context.Context) (iter.Seq[TestData], error) {
	return slices.Values(testData), nil
}

func TestLocalRepos(t *testing.T) {
	ctx := context.Background()

	t.Run("basic function test", func(t *testing.T) {
		repos := NewLocalRepos(mockDataStore, WithRefreshInterval(time.Second))

		repos.Start(ctx)
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, 3, repos.Len())

		values := repos.AllValues()
		assert.Equal(t, 3, len(values))

		keys := repos.AllKeys()
		assert.Equal(t, 3, len(keys))

		value := repos.Get("1")
		assert.Equal(t, "test1", value.value)

		items := repos.AllItems()
		assert.Equal(t, 3, len(items))

		err := repos.Close()
		assert.NoError(t, err)
	})

	t.Run("refresh interval test", func(t *testing.T) {
		repos := NewLocalRepos(mockDataStore, WithRefreshInterval(time.Second))
		repos.Start(ctx)

		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, 3, repos.Len())

		testData = append(testData, TestData{id: "4", value: "test4"})

		time.Sleep(2 * time.Second)

		assert.Equal(t, 4, repos.Len())

		repos.Close()
	})
}
