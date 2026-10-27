package peer_protocol

import "net"

// http://www.bittorrent.org/beps/bep_0010.html
type (
	ExtendedHandshakeMessage struct {
		M          map[ExtensionName]ExtensionNumber `bencode:"m"`
		V          string                            `bencode:"v,omitempty"`
		Reqq       int                               `bencode:"reqq,omitempty"`
		Encryption bool                              `bencode:"e,omitempty"`
		// BEP 9
		MetadataSize int `bencode:"metadata_size,omitempty"`
		// The local client port. It would be redundant for the receiving side of
		// a connection to send this.
		Port   int       `bencode:"p,omitempty"`
		YourIp CompactIp `bencode:"yourip,omitempty"`
		Ipv4   CompactIp `bencode:"ipv4,omitempty"`
		Ipv6   net.IP    `bencode:"ipv6,omitempty"`
	}

	ExtensionName   string
	ExtensionNumber int
)

const (
	// http://www.bittorrent.org/beps/bep_0011.html
	ExtensionNamePex ExtensionName = "ut_pex"
	// BEP 55. Relays a UDP dial when a direct connection fails.
	ExtensionNameHolepunch ExtensionName = "ut_holepunch"
	// Libtorrent: the peer will not download.
	ExtensionNameUploadOnly ExtensionName = "upload_only"
	// Libtorrent: the peer no longer has this piece.
	ExtensionNameDontHave ExtensionName = "lt_donthave"
	// http://bittorrent.org/beps/bep_0009.html. Note that there's an
	// LT_metadata, but I've never implemented it.
	ExtensionNameMetadata = "ut_metadata"
)
