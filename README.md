# Harvester 🌾

A super fast and lightweight BitTorrent client written in Go, designed for efficiency and simplicity while maintaining full protocol compatibility.

## Overview

Harvester is a minimal yet functional BitTorrent client implementation in Go. The project aims to create a complete, performant BitTorrent client with support for modern protocols including DHT (Distributed Hash Table) for decentralized peer discovery.

### Goals

-  **Performance**: Lightweight and fast downloads with minimal resource usage
-  **Full Protocol Support**: Complete implementation of the BitTorrent protocol specification
-  **DHT Support**: Decentralized peer discovery without reliance on centralized trackers
-  **Clean Codebase**: Well-organized, maintainable Go code following best practices
-  **Dependency Minimal**: Avoid unnecessary external dependencies


## Installation

### Requirements

- Go 1.24 or higher
- Linux, macOS, or Windows

### Build from Source

```bash
git clone https://github.com/m4kyu/harvester.git
cd harvester
go build -o harvester ./cmd/main.go
```

### Quick Start

```bash
# Download a torrent
./harvester path/to/file.torrent
```


## Protocol Support

- **BitTorrent Protocol v1** (BEP 3): Full core implementation
- **HTTP Trackers** (BEP 3, 15): Peer discovery via HTTP
- **DHT** (BEP 5): In development - decentralized peer discovery
- **Magnet Links** (BEP 9): Planned (requires DHT)

## Development Roadmap

### Phase 1: Core Implementation 
- [ ] Bencode support
- [x] Torrent file parsing
- [x] HTTP tracker communication
- [x] Peer wire protocol
- [x] Basic download functionality

### Phase 2: DHT & Decentralization  (Current)
- [ ] DHT network implementation
- [ ] Decentralized peer discovery
- [ ] Magnet link support
- [ ] Bootstrap and node management

### Phase 3: Enhancements & Optimization
- [x] Concurrent downloads
- [ ] Performance improvements
- [x] Extended protocol support (UDP trackers, PEX)
- [ ] Configuration system
- [ ] Rate limiting

### Phase 4: Stability & Polish
- [ ] Comprehensive testing
- [ ] Error handling and recovery
- [ ] Download resumption
- [ ] Documentation
- [ ] CLI refinement


## Performance Considerations

- **Memory Efficient**: Streams data directly without large buffers
- **Minimal Dependencies**: Pure Go implementation
- **Goroutine-based**: Efficient concurrent operations
- **Selective Downloading**: Choose which files to download in multi-file torrents (Planned)

## Security Considerations

- Validates all torrent metadata before starting download
- Verifies piece hashes against torrent info
- Sanitizes file paths to prevent directory traversal
- Optional encryption support (planned for future releases)

## Contributing

Contributions are welcome! Areas where help is needed:

1. **DHT Implementation**: Help implementing Distributed Hash Table
2. **Performance**: Optimization and profiling
3. **Testing**: Unit tests, integration tests, and edge cases
4. **Documentation**: Docstrings and guides
5. **Features**: Magnet links, PEX, UPnP support


## Known Limitations

- HTTP trackers only (UDP tracker support coming)
- No magnet link support (requires DHT)
- Limited error recovery and resilience
- No bandwidth throttling yet
- Requires full torrent metadata file


## References

- [BitTorrent Specification (BEP 3)](http://www.bittorrent.org/beps/bep_0003.html)
- [DHT Specification (BEP 5)](http://www.bittorrent.org/beps/bep_0005.html)
- [Bencode Specification](http://www.bittorrent.org/beps/bep_0003.html#bencoding)
- [Magnet Links (BEP 9)](http://www.bittorrent.org/beps/bep_0009.html)
- [PEX Protocol (BEP 11)](http://www.bittorrent.org/beps/bep_0011.html)

## License

This project is licensed under the **GNU General Public License v3.0** - see the [LICENSE](LICENSE) file for details.

## Author

Created and maintained by **m4kyu**

## Disclaimer

This tool is for educational and legal use only. Users are responsible for ensuring they have the legal right to download any content. Always respect copyright laws and distribution rights in your jurisdiction.

---

