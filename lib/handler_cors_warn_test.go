package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestNewHandlerWarnsWhenUnprotectedWithCORS ensures the unprotected-config
// warnings are emitted for every configuration, including when CORS is
// enabled. CORS is an early return in NewHandler, so an unauthenticated
// server behind a CORS configuration used to start without any warning.
func TestNewHandlerWarnsWhenUnprotectedWithCORS(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	defer zap.ReplaceGlobals(zap.New(core))()

	cfg := &Config{CORS: CORS{Enabled: true}}
	handler, err := NewHandler(cfg)
	require.NoError(t, err)
	require.NotNil(t, handler)
	require.NotZero(t, logs.Len(),
		"expected an unprotected-config warning for a CORS-enabled server without users")
}
