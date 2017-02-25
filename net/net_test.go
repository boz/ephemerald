package net_test

import (
	"fmt"
	"testing"

	"github.com/ovrclk/cleanroom/net"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {
	server, err := net.NewServerWithPort(0)

	require.NoError(t, err)

	readych := server.ServerReadyNotify()
	donech := server.ServerCloseNotify()
	defer func() {
		<-donech
	}()
	defer server.Close()

	go server.Run()
	<-readych
	fmt.Printf("server ready; port: %v\n", server.Port())

	client, err := net.NewClientBuilder().
		WithPort(server.Port()).
		Create()
	require.NoError(t, err)

	item, err := client.Redis().Checkout()
	require.NoError(t, err)
	require.NoError(t, client.Redis().Return(item))
}
