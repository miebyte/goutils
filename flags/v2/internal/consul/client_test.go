package consul

import (
	"testing"

	"github.com/miebyte/goutils/consulutils"
)

func TestDefaultProviderSharesPublicClient(t *testing.T) {
	client, err := consulutils.DefaultClient()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		provider, err := NewConsulClient()
		if err != nil {
			t.Fatal(err)
		}
		if provider.client != client {
			t.Fatal("configuration provider created a separate Consul client")
		}
		if err := provider.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
