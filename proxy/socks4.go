package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"
)

// DialSOCKS4 establishes a SOCKS4 TCP connection with context support.
func DialSOCKS4(ctx context.Context, proxyAddr, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Parse target address
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %s", portStr)
	}

	// Resolve host to IPv4 using context
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, fmt.Errorf("lookup host failed: %w", err)
	}

	var ipv4 net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4 = ip.To4()
			break
		}
	}
	if ipv4 == nil {
		return nil, fmt.Errorf("no IPv4 address found for %s", host)
	}

	// Dial TCP using context
	var dialer net.Dialer
	dialer.Timeout = timeout
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// Set temporary deadline for SOCKS4 handshake
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(timeout)
	}
	_ = conn.SetDeadline(deadline)
	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	// SOCKS4 request packet:
	// +----+----+----+----+----+----+----+----+----+----+....+----+
	// | VN | CD | DSTPORT |      DSTIP        | USERID       |NULL|
	// +----+----+----+----+----+----+----+----+----+----+....+----+
	//    1    1      2              4           variable       1
	req := make([]byte, 9)
	req[0] = 0x04 // VN: 4
	req[1] = 0x01 // CD: 1 (CONNECT)
	binary.BigEndian.PutUint16(req[2:4], uint16(port))
	copy(req[4:8], ipv4)
	req[8] = 0x00 // USERID: null-terminated empty string

	_, err = conn.Write(req)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// SOCKS4 reply packet:
	// +----+----+----+----+----+----+----+----+
	// | VN | CD | DSTPORT |      DSTIP        |
	// +----+----+----+----+----+----+----+----+
	//    1    1      2              4
	resp := make([]byte, 8)
	n, err := conn.Read(resp)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if n < 8 {
		conn.Close()
		return nil, fmt.Errorf("incomplete SOCKS4 reply: read %d bytes", n)
	}

	if resp[0] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("invalid SOCKS4 reply version: %d", resp[0])
	}

	switch resp[1] {
	case 0x5a:
		return conn, nil
	case 0x5b:
		conn.Close()
		return nil, fmt.Errorf("request rejected or failed")
	case 0x5c:
		conn.Close()
		return nil, fmt.Errorf("request rejected: client's identd not reachable")
	case 0x5d:
		conn.Close()
		return nil, fmt.Errorf("request rejected: identd user mismatch")
	default:
		conn.Close()
		return nil, fmt.Errorf("unknown SOCKS4 reply status: 0x%02x", resp[1])
	}
}
