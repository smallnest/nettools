# nettools

[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/baidu/nettools)](https://goreportcard.com/report/github.com/baidu/nettools)
[![Go Reference](https://pkg.go.dev/badge/github.com/baidu/nettools.svg)](https://pkg.go.dev/github.com/baidu/nettools)
[![CI](https://github.com/baidu/nettools/actions/workflows/ci.yml/badge.svg)](https://github.com/baidu/nettools/actions/workflows/ci.yml)

A suite of network diagnostic tools developed by Baidu's physical network black-box monitoring team, including:
- **bitflip**: Detects packet loss and bit-flip errors in large-scale physical networks.
- **bitflip6**: IPv6 variant of bitflip for IPv6 network diagnostics.
- More tools: TBD

> Produced by Baidu System Department


## bitflip

![](docs/bitflip.png)

A high-frequency UDP probing tool for network bit-flip (packet corruption) and packet loss detection. Supports **unidirectional** loss and corruption detection — both client-side (round-trip) and server-side (one-way from client to server).

**How it works:** The client sends a large volume of UDP packets per second to the server, which echoes them back unchanged. Both sides independently detect issues:

- **Client side (round-trip):** Detects packet loss and bit-flip on the full round-trip path. If a packet was dropped in either direction, it is counted as loss. If bit-flip is detected in the returned payload, the five-tuple is logged.
- **Server side (one-way):** Detects packet loss and bit-flip on the client-to-server direction only. Each packet carries the client's actual send count and starting port pair for the previous time window; the server uses these to compute one-way loss and reconstruct the complete set of expected port pairs — enabling per-five-tuple loss identification without tracking client state. The server auto-registers unknown clients on first packet — no pre-configuration required.

By comparing client-side and server-side loss, you can determine whether loss occurs on the **forward path** (client → server) or the **return path** (server → client).

### Quick Start

**Build:**
```bash
make build
```

**Run server (on the remote host):**
```bash
# Simplest — auto-detects local IP
./bitflip

# With explicit IP
./bitflip -r server -s <server_ip> -c <client_ip>
```

**Run client (on the local host):**
```bash
# -c auto-detected if empty, -s is required
./bitflip -r client -s <server_ip>

# With explicit IPs
./bitflip -r client -c <client_ip> -s <server_ip>
```

### Command-line Flags

| Short | Long | Default | Description |
|-------|------|---------|-------------|
| `-r` | `--role` | server | Role: client or server |
| `-c` | `--client-addr` | "" | Client IP address (auto-detected if empty) |
| `-s` | `--server-addr` | "" | Server IP address (auto-detected for server role if empty) |
| `-t` | `--tos` | 64 | IP TOS/DSCP value |
| `-n` | `--count` | 0 | Max packets to send (0 = unlimited) |
| `-d` | `--duration` | 0 | Max send duration (0 = unlimited) |
| | `--client-ports` | "43500,43599" | Client port range [min,max] |
| | `--server-ports` | "43500,43509" | Server port range [min,max] |
| | `--rate` | 5000 | Packets per span |
| | `--msglen` | 1024 | Message payload size (excluding 32-byte header) |
| | `--delay` | 3s | Delay before processing stats (waiting for in-flight packets) |
| | `--verbose` | false | Print per-port loss details on packet loss (both client and server) |

### Examples

```bash
# Server side — auto-detect, no need to specify -c
# Server auto-registers unknown clients on first packet
./bitflip

# Client side
sudo ./bitflip -r client -s 10.0.0.2

# Client with custom rate and duration
sudo ./bitflip --role client --server-addr 10.0.0.2 --rate 10000 --duration 60s

# Client with verbose loss port details
sudo ./bitflip -r client -s 10.0.0.2 --verbose

# Server with verbose loss port details (per-five-tuple loss)
./bitflip -s 10.0.0.1 --verbose
```

### Bit-flip Detection

The client sends packets padded with 4 salt patterns, selected by `seq % 4`:

| Index | Pattern | Description |
|-------|---------|-------------|
| 0 | `0xFF` | All-ones byte |
| 1 | `0x00` | All-zeros byte |
| 2 | `0x5A` | Fixed pattern `01011010` |
| 3 | Complementary alternating | `0xAAAA` / `0x5555` alternating 16-bit words |

The server uses the same 4 salt patterns to validate packets, ensuring accurate identification of which bytes have been flipped.

### Packet Format

```
+----------+----------+-----------+---------------+------------------+------------------+----------+
| Magic(8) | Seq(8)   | Ts(8)     | LastSent(4)   | LastSrcPort(2)   | LastDstPort(2)   | Salt(N)  |
+----------+----------+-----------+---------------+------------------+------------------+----------+
```

- **Magic**: 8-byte magic flag identifier
- **Seq**: 8-byte sequence number
- **Ts**: 8-byte nanosecond timestamp
- **LastSent**: 4-byte previous span send count
- **LastSrcPort**: 2-byte previous span starting source port
- **LastDstPort**: 2-byte previous span starting destination port
- **Salt**: N-byte padding data (for bit-flip detection)

Through this compact protocol design, the server can reconstruct every port pair from the previous span using `(LastSrcPort, LastDstPort, LastSent)` and the deterministic `GetNextPorts` algorithm — enabling **server-side unidirectional loss detection with per-five-tuple granularity**, without the server needing to track client-side send state.

## bitflip6

IPv6 variant of bitflip. Usage is identical to bitflip, with IPv6 addresses:

```bash
# Server side
./bitflip6

# Client side
sudo ./bitflip6 -r client -s fd00::2
```

## Testing

```bash
make test
```

## Test Coverage

![](coverage.svg)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## License

This project is licensed under the [MIT License](LICENSE).
