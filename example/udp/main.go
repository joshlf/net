package main

import (
	"bufio"
	"fmt"
	gonet "net"
	"os"

	"github.com/joshlf/net"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <laddr> <raddr>\n", os.Args[0])
		os.Exit(1)
	}
	laddr, err := gonet.ResolveUDPAddr("udp4", os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve laddr: ", err)
		os.Exit(2)
	}
	raddr, err := gonet.ResolveUDPAddr("udp4", os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve raddr: ", err)
		os.Exit(2)
	}
	dev, err := net.NewUDPIPv4Device(laddr, raddr, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create device: ", err)
		os.Exit(2)
	}
	err = dev.SetIPv4(net.IPv4{10, 0, 0, 2}, net.IPv4{255, 0, 0, 0})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	err = dev.BringUp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var host net.IPv4Host
	host.AddDevice(dev)
	for i := 0; i < 256; i++ {
		proto := net.IPv4Protocol(i)
		host.RegisterCallback(func(b []byte, src, dst net.IPv4) {
			callback(b, src, dst, proto)
		}, proto)
	}
	host.AddDeviceRoute(net.IPv4Subnet{
		Addr:    net.IPv4{10, 0, 0, 0},
		Netmask: net.IPv4{255, 0, 0, 0},
	}, dev)

	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		line := s.Text()
		_, err = host.WriteTo([]byte(line), net.IPv4{10, 0, 0, 1}, 1)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func callback(b []byte, src, dst net.IPv4, proto net.IPv4Protocol) {
	fmt.Printf("%v -> %v (%v): %v\n", src, dst, proto, string(b))
}
