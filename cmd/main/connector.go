package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type UDPConnector struct {
	addr *net.UDPAddr
	con  *net.UDPConn
}

func NewUDPConnector(localAddr string) (*UDPConnector, error) {
	addr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	return &UDPConnector{
		addr: addr,
		con:  conn,
	}, nil
}
func (c *UDPConnector) SetDestinationTo(localAddr string) error {
	addr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return err
	}
	c.addr = addr
	return nil
}
func (c *UDPConnector) SetDestination(addr *net.UDPAddr) {
	c.addr = addr
}
func (c *UDPConnector) Recv(size int32, timeout float32) ([]byte, *net.UDPAddr, error) {
	buf := make([]byte, size)
	err := c.con.SetReadDeadline(time.Now().Add(time.Duration(float32(time.Second) * timeout)))
	if err != nil {
		return nil, nil, err
	}
	n, addr, err := c.con.ReadFromUDP(buf)
	if err != nil {
		return nil, nil, err
	}
	return buf[:n], addr, nil
}
func (c *UDPConnector) Send(buf []byte) error {
	if c.addr == nil {
		return fmt.Errorf("no destination address set")
	}
	c.con.WriteToUDP(buf, c.addr)
	return nil
}
func (c *UDPConnector) Close() error {
	if c.con != nil {
		return c.con.Close()
	}
	return nil
}

type TCPConnector struct {
	addr *net.TCPAddr
	con  *net.TCPConn
}

func NewTCPConnector(p *Peer) *TCPConnector {
	addr := net.TCPAddr{
		IP:   p.IP,
		Port: int(p.port),
	}
	tcon := TCPConnector{}
	tcon.SetDestination(&addr)
	return &tcon
}
func (c *TCPConnector) SetDestinationTo(remoteAddr string) error {
	addr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return err
	}
	c.SetDestination(addr)
	return nil
}
func (c *TCPConnector) SetDestination(addr *net.TCPAddr) {
	if c.con != nil {
		c.con.Close()
		c.con = nil
	}
	c.addr = addr
}
func (c *TCPConnector) Send(buf []byte) error {
	if c.con == nil {
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.Dial("tcp", c.addr.String())
		if err != nil {
			return err
		}
		c.con = conn.(*net.TCPConn)
	}
	_, err := c.con.Write(buf)
	return err
}
func (c *TCPConnector) Recv(size int32, timeout float32) ([]byte, *net.TCPAddr, error) {
	if c.con == nil {
		return nil, nil, fmt.Errorf("no active connection")
	}
	buf := make([]byte, size)
	deadline := time.Now().Add(time.Duration(timeout * float32(time.Second)))
	err := c.con.SetReadDeadline(deadline)
	if err != nil {
		return nil, nil, err
	}
	n, err := c.con.Read(buf)
	if err != nil {
		return nil, nil, err
	}
	return buf[:n], c.addr, nil
}

func (c *TCPConnector) RecvAll(size int32, timeout float32) ([]byte, *net.TCPAddr, error) {
	if c.con == nil {
		return nil, nil, fmt.Errorf("no active connection")
	}
	buf := make([]byte, size)
	deadline := time.Now().Add(time.Duration(timeout * float32(time.Second)))
	err := c.con.SetReadDeadline(deadline)
	if err != nil {
		return nil, nil, err
	}
	n, err := io.ReadFull(c.con, buf)
	if err != nil {
		return nil, nil, err
	}
	return buf[:n], c.addr, nil
}
func (c *TCPConnector) Close() error {
	if c.con != nil {
		return c.con.Close()
	}
	return nil
}

type HTTPConnector struct {
	client  *http.Client
	baseURL string
}

func NewHTTPConnector(baseURL string) *HTTPConnector {
	return &HTTPConnector{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}
