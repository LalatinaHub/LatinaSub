package netutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	require.NoError(t, err)
	assert.Greater(t, port, 1024)
	assert.LessOrEqual(t, port, 65535)

	mustPort := MustGetFreePort()
	assert.Greater(t, mustPort, 1024)
	assert.LessOrEqual(t, mustPort, 65535)
}

func TestConcurrentGetFreePort(t *testing.T) {
	const n = 10
	ports := make(chan int, n)

	for i := 0; i < n; i++ {
		go func() {
			port, err := GetFreePort()
			if err == nil {
				ports <- port
			} else {
				ports <- 0
			}
		}()
	}

	for i := 0; i < n; i++ {
		port := <-ports
		assert.Greater(t, port, 0)
	}
}

func TestAllocatePort(t *testing.T) {
	port, release, err := AllocatePort()
	require.NoError(t, err)
	assert.Greater(t, port, 1024)
	assert.NotNil(t, release)
	release()
}


