package testharness

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

// TCPProxy
//
//		this TCP proxy not intended for production use
//	 possible connection leakage (look at the `tunnel` structure)
type TCPProxy struct {
	serverBind string
	target     string
	cancel     context.CancelFunc
}

func (tcpp *TCPProxy) pipe(ctx context.Context, from net.Conn, to net.Conn, wg *sync.WaitGroup) {
	fmt.Printf("handle data flow from=(%v,%v) to=(%v,%v)\n", from.LocalAddr(), from.RemoteAddr(), to.LocalAddr(), to.RemoteAddr())
	defer func() {
		fmt.Printf("end serve data pipe from: %v, to %v\n", from, to)
		wg.Done()
	}()

	b := make([]byte, 10240)
	for {
		n, err := from.Read(b)
		if tcpp.contextCancelled(ctx) {
			fmt.Println("exit pipe due context cancel")
			return
		}

		if err != nil {
			fmt.Printf("input error: %s\n", err)
			return
		}

		if n > 0 {
			to.Write(b[:n])
		}
	}
}

func (tcpp *TCPProxy) handleConnection(ctx context.Context, local net.Conn, remote net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go tcpp.pipe(ctx, remote, local, &wg)
	go tcpp.pipe(ctx, local, remote, &wg)

	wg.Wait()
	fmt.Printf("data transfer pipes closed\n")
	remote.Close()
	local.Close()
}

func NewTCPProxy(serverBind string, target string) *TCPProxy {
	return &TCPProxy{serverBind, target, nil}
}

type tunnel struct {
	from net.Conn
	to   net.Conn
}

func (tcpp *TCPProxy) Start(ctx context.Context) error {
	ctx, tcpp.cancel = context.WithCancel(ctx)

	ln, err := net.Listen("tcp", tcpp.serverBind)
	if err != nil {
		return fmt.Errorf("error start listener: %w", err)
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	go func() {
		fmt.Printf("start accepting connections\n")
		var tunnels []tunnel
		defer func() {
			for _, tunnel := range tunnels {
				tunnel.from.Close()
				tunnel.to.Close()
			}
			fmt.Printf("stop accepting connections\n")
		}()

		for {
			if conn, err := ln.Accept(); err == nil {
				fmt.Printf("handle connection: %v\n", conn)

				fmt.Printf("connect to remote host: %v\n", tcpp.target)
				remote, err := net.Dial("tcp", tcpp.target)
				if err != nil {
					fmt.Printf("Unable to connect to %s, %s\n", tcpp.target, err)
					continue
				}
				// this is simple as possible implementation, without eviction of already closed connections pair
				// this TCP proxy not intended for production use
				tunnels = append(tunnels, tunnel{conn, remote})
				go tcpp.handleConnection(ctx, conn, remote)
			} else {
				if tcpp.contextCancelled(ctx) {
					return
				}

				fmt.Printf("Accept failed, %v\n", err)
			}
		}
	}()
	return nil
}

func (tcpp *TCPProxy) Close() error {
	if tcpp.cancel == nil {
		return fmt.Errorf("proxy not yet started")
	}

	fmt.Printf("stop tcp proxy\n")
	tcpp.cancel()
	tcpp.cancel = nil
	return nil
}

func (tcpp *TCPProxy) contextCancelled(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}
