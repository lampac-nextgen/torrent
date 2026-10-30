package torrent

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/dht/v2/krpc"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/stretchr/testify/assert"
)

func TestNextDhtAnnounceInterval(t *testing.T) {
	for _, c := range []struct {
		name     string
		current  time.Duration
		hasPeers bool
		want     time.Duration
	}{
		{"healthy keeps regular", dhtAnnounceRetryInterval, true, dhtAnnounceInterval},
		{"healthy from regular", dhtAnnounceInterval, true, dhtAnnounceInterval},
		{"first empty announce speeds up to retry", dhtAnnounceInterval, false, dhtAnnounceRetryInterval},
		{"retry doubles", dhtAnnounceRetryInterval, false, 2 * dhtAnnounceRetryInterval},
		{"doubling continues", 2 * dhtAnnounceRetryInterval, false, 4 * dhtAnnounceRetryInterval},
		{"doubling caps at regular", 4 * time.Minute, false, dhtAnnounceInterval},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, nextDhtAnnounceInterval(c.current, c.hasPeers))
		})
	}
}

type stubDhtAnnounce struct {
	peers <-chan dht.PeersValues
}

func (s stubDhtAnnounce) Close() {}

func (s stubDhtAnnounce) Peers() <-chan dht.PeersValues { return s.peers }

// stubDhtServer is a DhtServer whose Announce finishes immediately with no
// peers, mimicking a traversal that reaches the swarm but returns empty.
type stubDhtServer struct{}

func (stubDhtServer) Stats() interface{}          { return nil }
func (stubDhtServer) ID() [20]byte                { return [20]byte{} }
func (stubDhtServer) Addr() net.Addr              { return nil }
func (stubDhtServer) AddNode(krpc.NodeInfo) error { return nil }
func (stubDhtServer) Ping(*net.UDPAddr)           {}
func (stubDhtServer) WriteStatus(io.Writer)       {}
func (stubDhtServer) Announce([20]byte, int, bool) (DhtAnnounce, error) {
	ch := make(chan dht.PeersValues)
	close(ch)
	return stubDhtAnnounce{peers: ch}, nil
}

func TestAnnounceToDhtReturnsWhenTraversalEnds(t *testing.T) {
	cfg := NewDefaultClientConfig()
	cfg.ListenPort = 0
	cfg.NoDHT = true
	cfg.NoDefaultPortForwarding = true
	cl, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	tr, _ := cl.AddTorrentInfoHash(metainfo.Hash{})

	start := time.Now()
	if err := tr.announceToDht(stubDhtServer{}); err != nil {
		t.Fatal(err)
	}
	assert.Less(t, time.Since(start), 3*time.Second,
		"announceToDht must return when the traversal ends, not wait the full regular interval")
}

// AnnounceToDht reads the listen port through incomingPeerPort, which walks cl.conns.
// Client.Close sets cl.conns to nil under the client lock, so the read must hold it too.
// Fails under -race when dhtAnnouncer's announce races a Close (seen as a flake at
// TorrServer's Disconnect: closeSockets write against LocalPort read).
func TestAnnounceToDhtPortReadIsLockedAgainstClose(t *testing.T) {
	cfg := NewDefaultClientConfig()
	cfg.ListenPort = 0
	cfg.NoDHT = true
	cfg.NoDefaultPortForwarding = true
	cl, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	tr, _ := cl.AddTorrentInfoHash(metainfo.Hash{})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = tr.announceToDht(stubDhtServer{})
			}
		}
	}()
	time.Sleep(20 * time.Millisecond) // let announces run
	cl.Close()
	close(stop)
	<-done
}
