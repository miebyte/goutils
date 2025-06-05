package dialer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestService struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func TestDialService(t *testing.T) {
	serviceConf = ServiceConfigMap{
		"test_service": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
		},
		"invalid_service": "invalid_service",
	}
	isInit = true

	t.Run("get service config", func(t *testing.T) {
		service, err := DialService[TestService]("test_service")
		assert.NoError(t, err)
		assert.Equal(t, "localhost", service.Host)
		assert.Equal(t, 8080, service.Port)
	})

	t.Run("service not exists", func(t *testing.T) {
		_, err := DialService[TestService]("nonexistent_service")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrServiceNotExists)
	})

	t.Run("invalid service config", func(t *testing.T) {
		_, err := DialService[TestService]("invalid_service")
		assert.Error(t, err)
	})

	t.Run("service config not init", func(t *testing.T) {
		oldIsInit := isInit
		isInit = false
		defer func() {
			isInit = oldIsInit
		}()

		_, err := DialService[TestService]("test_service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not init")
	})
}
