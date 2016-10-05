package main

import (
	"bufio"
	"fmt"
	gonet "net"
	"os"
	"strconv"
	"strings"

	"github.com/joshlf/net"
	"github.com/joshlf/net/example/internal"
	"github.com/juju/errors"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <devices-file> <routes-file>\n", os.Args[0])
		os.Exit(1)
	}
	devfile, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "open devices file: ", err)
		os.Exit(2)
	}
	defs, err := internal.ParsePairs(devfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse devices file: ", err)
		os.Exit(2)
	}

	var host net.IPv4Host
	host.SetForwarding(true)
	var devset net.DeviceSet
	for _, def := range defs {
		dev, err := definitionToDevice(def[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		err = dev.BringUp()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		devset.Put(def[0], dev)
		host.AddDevice(dev)
	}

	routefile, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "open routes file: ", err)
		os.Exit(2)
	}
	routes, err := internal.ParseRouteFile(routefile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse routes file: ", err)
		os.Exit(2)
	}
	for _, route := range routes {
		switch route := route.(type) {
		case internal.RouteEntry:
			if route.Subnet.IPVersion() != 4 {
				fmt.Fprintln(os.Stderr, "cannot use IPv6 route ", route)
				os.Exit(2)
			}
			host.AddRoute(route.Subnet.(net.IPv4Subnet), route.Nexthop.(net.IPv4))
		case internal.RouteDeviceEntry:
			if route.Subnet.IPVersion() != 4 {
				fmt.Fprintln(os.Stderr, "cannot use IPv6 device route ", route)
				os.Exit(2)
			}
			dev, ok := devset.Get(route.Device)
			if !ok {
				fmt.Fprintln(os.Stderr, "no such device: ", route.Device)
				os.Exit(2)
			}
			host.AddDeviceRoute(route.Subnet.(net.IPv4Subnet), dev.(net.IPv4Device))
		default:
			panic("unreachable")
		}
	}

	for i := 0; i < 256; i++ {
		proto := net.IPv4Protocol(i)
		host.RegisterCallback(func(b []byte, src, dst net.IPv4) {
			callback(b, src, dst, proto)
		}, proto)
	}
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 0 {
			continue
		}
		ip, err := net.ParseIPv4(fields[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		_, err = host.WriteTo([]byte(strings.Join(fields[1:], "1")), ip, 1)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func definitionToDevice(s string) (*net.UDPIPv4Device, error) {
	fields := strings.Fields(s)
	if len(fields) != 4 {
		return nil, errors.Errorf("parse device definition: unexpected number of whitespace-separated fields: %v", len(fields))
	}
	addr, subnet, err := net.ParseCIDRIPv4(fields[0])
	if err != nil {
		return nil, errors.Annotate(err, "parse device definition")
	}
	laddr, err := gonet.ResolveUDPAddr("udp", fields[1])
	if err != nil {
		return nil, errors.Annotate(err, "create device from definition")
	}
	raddr, err := gonet.ResolveUDPAddr("udp", fields[2])
	if err != nil {
		return nil, errors.Annotate(err, "create device from definition")
	}
	mtu, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil, errors.Annotate(err, "parse device definition: parse MTU")
	}
	dev, err := net.NewUDPIPv4Device(laddr, raddr, mtu)
	if err != nil {
		return nil, errors.Annotate(err, "create device from definition")
	}
	err = dev.SetIPv4(addr, subnet.Netmask)
	return dev, errors.Annotate(err, "create device from definition")
	// return dev, errors.Annotate(err, "create device from definition")
}

func main1() {
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
