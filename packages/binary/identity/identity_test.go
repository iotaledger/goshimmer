package identity

import (
	"sync"
	"testing"

	"github.com/panjf2000/ants/v2"

	"github.com/stretchr/testify/assert"
)

func BenchmarkIdentity_VerifySignature(b *testing.B) {
	identity := Generate()
	data := []byte("TESTDATA")
	signature := identity.Sign(data)

	var wg sync.WaitGroup

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wg.Add(1)

		_ = ants.Submit(func() {
			identity.VerifySignature(data, signature)

			wg.Done()
		})
	}

	wg.Wait()
}

func Test(t *testing.T) {
	identity := Generate()

	signature := identity.Sign([]byte("TESTDATA1"))

	assert.Equal(t, true, identity.VerifySignature([]byte("TESTDATA1"), signature))
	assert.Equal(t, false, identity.VerifySignature([]byte("TESTDATA2"), signature))
}
