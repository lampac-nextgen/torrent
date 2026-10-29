package torrent

import (
	"net"
	"strconv"
	"testing"

	"github.com/anacrolix/utp"
)

func utpOnlyConfig(t *testing.T, port int) *ClientConfig {
	cfg := NewDefaultClientConfig()
	cfg.DataDir = t.TempDir()
	cfg.NoDHT = true
	cfg.DisablePEX = true
	cfg.DisableTCP = true
	cfg.DisableIPv6 = true
	cfg.NoDefaultPortForwarding = true
	cfg.LocalServiceDiscovery = nil
	cfg.ListenHost = func(string) string { return "127.0.0.1" }
	cfg.ListenPort = port
	return cfg
}

// Client.Close must free the uTP port even while a peer conn on it is still open,
// so a client restarted on the same ListenPort can bind.
func TestCloseReleasesUtpPortWithLiveConn(t *testing.T) {
	cl, err := NewClient(utpOnlyConfig(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	port := cl.LocalPort()

	peer, err := utp.NewSocket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	conn, err := peer.Dial(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	cl.Close()

	again, err := NewClient(utpOnlyConfig(t, port))
	if err != nil {
		t.Fatalf("restart on port %d: %v", port, err)
	}
	again.Close()
}
