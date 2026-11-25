package cmd

import (
	"net"
	"testing"

	"github.com/hacdias/webdav/v5/lib"
	"github.com/stretchr/testify/require"
)

func TestGetListenerIPv6(t *testing.T) {
	probe, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback")
	}
	require.NoError(t, probe.Close())

	listener, err := getListener(&lib.Config{Address: "::1", Port: 0})
	require.NoError(t, err)
	require.Equal(t, "::1", listener.Addr().(*net.TCPAddr).IP.String())
	require.NoError(t, listener.Close())
}
