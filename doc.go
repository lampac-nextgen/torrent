/*
Package torrent implements a torrent client. Goals include:
  - Configurable data storage, such as file, mmap, and piece-based.
  - Downloading on demand: torrent.Reader will request only the data required to
    satisfy Reads, which is ideal for streaming and torrentfs.

BitTorrent features implemented include:
  - Protocol obfuscation
  - DHT
  - uTP
  - PEX
  - Magnet links
  - IP Blocklists
  - Some IPv6
  - HTTP and UDP tracker clients
  - BEPs:
  - 3: Basic BitTorrent protocol
  - 5: DHT
  - 6: Fast Extension (HaveAll/HaveNone/Reject/AllowedFast, Suggest inside the reader window)
  - 7: IPv6 Tracker Extension
  - 9: ut_metadata
  - 10: Extension protocol
  - 11: PEX
  - 12: Multitracker metadata extension
  - 15: UDP Tracker Protocol
  - 20: Peer ID convention ("-GTnnnn-")
  - 23: Tracker Returns Compact Peer Lists
  - 27: Private torrents
  - 29: uTorrent transport protocol
  - 40: Canonical Peer Priority
  - 41: UDP Tracker Protocol Extensions
  - 42: DHT Security extension
  - 43: Read-only DHT Nodes
  - 55: Holepunch (ut_holepunch)
  - 19: WebSeed HTTP fetch (url-list / magnet ws)
  - Libtorrent LTEP: upload_only, lt_donthave

Not implemented, on purpose: BEP 52/53 (BitTorrent v2 and magnet v2). v1 swarms
still carry almost all TorrServer playback. Revisit when hybrid or v2 magnets
fail in the wild often enough to justify merkle piece layers. WebRTC WebTorrent
peers are also out of scope.
*/
package torrent
