package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joshlf/net"
	"github.com/joshlf/net/example/internal"
	"github.com/joshlf/net/example/internal/cli"
	"github.com/spf13/pflag"
)

var (
	routeFileFlag  string
	forwardingFlag bool

	host = net.IPHost{
		IPv4: &net.IPv4Host{},
		IPv6: &net.IPv6Host{},
	}
)

func init() {
	pflag.StringVar(&routeFileFlag, "route-file", "", "File with route table.")
	pflag.BoolVar(&forwardingFlag, "ip-forward", false, "Turn on IP forwarding.")
	postParseFuncs = append(postParseFuncs, func() {
		if !pflag.Lookup("route-file").Changed {
			fmt.Fprintln(os.Stderr, "Missing required flag --route-file")
			os.Exit(1)
		}

		host.SetForwarding(forwardingFlag)

		routefile, err := os.Open(routeFileFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "open route file:", err)
			os.Exit(2)
		}
		routes, err := internal.ParseRouteFile(routefile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse route file:", err)
			os.Exit(2)
		}
		for _, route := range routes {
			switch route := route.(type) {
			case internal.RouteEntry:
				if route.Subnet.IPVersion() != 4 {
					fmt.Fprintln(os.Stderr, "cannot use IPv6 route", route)
					os.Exit(2)
				}
				host.AddRoute(route.Subnet.(net.IPv4Subnet), route.Nexthop.(net.IPv4))
			case internal.RouteDeviceEntry:
				if route.Subnet.IPVersion() != 4 {
					fmt.Fprintln(os.Stderr, "cannot use IPv6 device route", route)
					os.Exit(2)
				}
				dev, ok := devices.Get(route.Device)
				if !ok {
					fmt.Fprintln(os.Stderr, "no such device:", route.Device)
					os.Exit(2)
				}
				host.AddDeviceRoute(route.Subnet.(net.IPv4Subnet), dev.(net.IPv4Device))
			default:
				panic("unreachable")
			}
		}
	})
}

var cmdIP = cli.Command{
	Name:             "ip",
	ShortDescription: "IP-related commands",
	LongDescription:  "IP-related commands.",
}

var cmdIPListen = cli.Command{
	Name:             "listen",
	Usage:            "<protocol number> [on | off]",
	ShortDescription: "Listen for IP packets",
	LongDescription: `Turn listening on or off for IP packets with a given protocol number.
When listening is on, IP packets received with the given protocol number
will be printed to the terminal.`,

	Run: func(cmd *cli.Command, args []string) {
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			cmd.PrintUsage()
			return
		}

		proto, err := strconv.ParseUint(args[0], 10, 8)
		if err != nil {
			fmt.Println("could not parse protocol number:", err)
			return
		}

		if args[1] == "on" {
			f := func(b []byte, src, dst net.IP) {
				fmt.Printf("%v -> %v (%v): %v\n", src, dst, proto, string(b))
			}
			host.IPv4.RegisterCallback(func(b []byte, src, dst net.IPv4) {
				f(b, src, dst)
			}, net.IPProtocol(proto))
			host.IPv6.RegisterCallback(func(b []byte, src, dst net.IPv6) {
				f(b, src, dst)
			}, net.IPProtocol(proto))
		} else {
			host.IPv4.RegisterCallback(nil, net.IPProtocol(proto))
			host.IPv6.RegisterCallback(nil, net.IPProtocol(proto))
		}
	},
}

var cmdIPSend = cli.Command{
	Name:             "send",
	Usage:            "<destination> <protocol number> [--ttl <ttl>] [<body>...]",
	ShortDescription: "Send an IP packet",
	LongDescription: `Send an IP packet to the given destination with the specified
protocol number and body. The body may consist of 0 or more elements,
each of which will be joined with a single space character.`,

	Run: func(cmd *cli.Command, args []string) {
		if len(args) < 2 || (len(args) == 3 && args[2] == "--ttl") {
			// (len(args) == 3 && args[2] == "--ttl") is true when
			// --ttl is the last argument (and thus must be an
			// illegal usage)
			cmd.PrintUsage()
			return
		}

		dst, err := net.ParseIP(args[0])
		if err != nil {
			fmt.Println("could not parse destination IP:", err)
			return
		}
		proto, err := strconv.ParseUint(args[1], 10, 8)
		if err != nil {
			fmt.Println("could not parse protocol number:", err)
			return
		}
		if len(args) > 3 && args[2] == "--ttl" {
			ttl, err := strconv.ParseUint(args[3], 10, 8)
			if err != nil {
				fmt.Println("could not parse ttl:", err)
				return
			}
			_, err = host.WriteToTTL([]byte(strings.Join(args[4:], " ")), dst, net.IPProtocol(proto), uint8(ttl))
			if err != nil {
				fmt.Println("could not send:", err)
			}
		} else {
			_, err := host.WriteTo([]byte(strings.Join(args[2:], " ")), dst, net.IPProtocol(proto))
			if err != nil {
				fmt.Println("could not send:", err)
			}
		}
	},
}

var cmdIPForward = cli.Command{
	Name:             "forward",
	Usage:            "[on | off]",
	ShortDescription: "Get or set IP forwarding state",
	LongDescription: `Without any arguments, display the state of IP forwarding.
With a single argument, "on" or "off", turn IP forwarding on or off.`,

	Run: func(cmd *cli.Command, args []string) {
		switch {
		case len(args) == 0:
			on := "on"
			if !host.IPv4.Forwarding() {
				// since we only ever turn forwarding on/off for both
				// IPv4 and IPv6 at the same time, just check IPv4
				on = "off"
			}
			fmt.Println("IP forwarding is", on)
		case len(args) == 1 && args[0] == "on":
			host.SetForwarding(true)
		case len(args) == 1 && args[0] == "off":
			host.SetForwarding(false)
		default:
			cmd.PrintUsage()
		}
	},
}

var cmdIPRoute = cli.Command{
	Name:             "route",
	ShortDescription: "view and manipulate the IP routing table",
	LongDescription:  "View and manipulate the IP routing table.",

	Run: func(cmd *cli.Command, args []string) {
		if len(args) > 0 {
			fmt.Println("Usage: ip route show")
			return
		}

		ipv4Routes := host.IPv4.Routes()
		ipv4DevRoutes := host.IPv4.DeviceRoutes()
		ipv6Routes := host.IPv6.Routes()
		ipv6DevRoutes := host.IPv6.DeviceRoutes()
		fmt.Println("IPv4 Routes")
		printIPv4Routes(ipv4Routes, ipv4DevRoutes)
		fmt.Println("IPv6 Routes")
		fmt.Println("===========")
		for _, r := range ipv6Routes {
			fmt.Printf("%v %v %v\n", r.Subnet.Addr, r.Subnet.Netmask, r.Nexthop)
		}
		for _, r := range ipv6DevRoutes {
			name, ok := devices.GetName(r.Device)
			if !ok {
				panic(fmt.Errorf("unexpected internal error: could not get name for device %v", r.Device))
			}
			fmt.Printf("%v %v %v\n", r.Subnet.Addr, r.Subnet.Netmask, name)
		}
		return
	},
}

func printIPv4Routes(routes []net.IPv4Route, devroutes []net.IPv4DeviceRoute) {
	// at least three spaces between each element on a line
	fmt.Println("Address           Netmask           Next Hop")
	fmt.Println("============================================")

	const maxlen = len("000.000.000.000")
	for _, r := range routes {
		addr := fmt.Sprint(r.Subnet.Addr)
		addr += strings.Repeat(" ", maxlen-len(addr))
		netmask := fmt.Sprint(r.Subnet.Netmask)
		netmask += strings.Repeat(" ", maxlen-len(netmask))
		fmt.Printf("%v   %v   %v\n", addr, netmask, r.Nexthop)
	}
	for _, r := range devroutes {
		addr := fmt.Sprint(r.Subnet.Addr)
		addr += strings.Repeat(" ", maxlen-len(addr))
		netmask := fmt.Sprint(r.Subnet.Netmask)
		netmask += strings.Repeat(" ", maxlen-len(netmask))
		name, ok := devices.GetName(r.Device)
		if !ok {
			panic(fmt.Errorf("unexpected internal error: could not get name for device %v", r.Device))
		}
		fmt.Printf("%v   %v   %v\n", addr, netmask, name)
	}
}

// sortableRoutes sorts routes by subnet; normal and device routes
// may be mixed, but IPv4 and IPv6 routes may not be mixed
type sortableRoutes []interface{}

func (s sortableRoutes) Len() int      { return len(s) }
func (s sortableRoutes) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableRoutes) Less(i, j int) bool {
	si, sj := s[i], s[j]
	_, ok4Route := si.(net.IPv4Route)
	_, ok4DevRoute := si.(net.IPv4DeviceRoute)
	if ok4Route || ok4DevRoute {
		var subneti, subnetj net.IPv4Subnet
		switch si := si.(type) {
		case net.IPv4Route:
			subneti = si.Subnet
		case net.IPv4DeviceRoute:
			subneti = si.Subnet
		}
		switch sj := sj.(type) {
		case net.IPv4Route:
			subnetj = sj.Subnet
		case net.IPv4DeviceRoute:
			subnetj = sj.Subnet
		}
		// TODO(joshlf): Compare them
		_, _ = subneti, subnetj
	} else {
		var subneti, subnetj net.IPv6Subnet
		switch si := si.(type) {
		case net.IPv6Route:
			subneti = si.Subnet
		case net.IPv6DeviceRoute:
			subneti = si.Subnet
		}
		switch sj := sj.(type) {
		case net.IPv6Route:
			subnetj = sj.Subnet
		case net.IPv6DeviceRoute:
			subnetj = sj.Subnet
		}
		// TODO(joshlf): Compare them
		_, _ = subneti, subnetj
	}
	panic("not implemented")
}

func init() {
	topLevelCommands = append(topLevelCommands, &cmdIP)
	cmdIP.AddSubcommand(&cmdIPListen)
	cmdIP.AddSubcommand(&cmdIPSend)
	cmdIP.AddSubcommand(&cmdIPForward)
	cmdIP.AddSubcommand(&cmdIPRoute)
}
