package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/joshlf/net"
	"github.com/joshlf/net/example/internal"
	"github.com/joshlf/net/example/internal/cli"
	"github.com/spf13/pflag"
)

var (
	deviceFileFlag string

	deviceDrivers = make(map[string]*deviceDriver)
	devices       net.DeviceSet
)

type deviceDriver struct {
	getDevice func(args []string) (net.Device, error)
	// get any information relevant for printing using 'dev' command
	getInfo func(dev net.Device) (string, error)

	// use to initialize driver-type-specific commands
	// (doesn't really need to be concurrency safe;
	// sync.Once is just a handy primitive to use here)
	once sync.Once
	init func()
}

func init() {
	pflag.StringVar(&deviceFileFlag, "device-file", "", "File with device definitions.")
	postParseFuncs = append(postParseFuncs, func() {
		if !pflag.Lookup("device-file").Changed {
			fmt.Fprintln(os.Stderr, "Missing required flag --device-file")
			os.Exit(1)
		}

		devfile, err := os.Open(deviceFileFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not open device file:", err)
			os.Exit(2)
		}
		pairs, err := internal.ParsePairs(devfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not parse device file:", err)
			os.Exit(2)
		}
		for _, pair := range pairs {
			typ, name, err := parseDevName(pair[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not parse device name %q: %v\n", pair[0], err)
				os.Exit(1)
			}
			driver, ok := deviceDrivers[typ]
			if !ok {
				fmt.Fprintf(os.Stderr, "%v: no such device driver\n", typ)
				os.Exit(1)
			}
			driver.once.Do(driver.init)
			dev, err := driver.getDevice(strings.Fields(pair[1]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not initialize device %v: %v\n", name, err)
				os.Exit(1)
			}
			err = dev.BringUp()
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not bring up device %v: %v\n", name, err)
				os.Exit(1)
			}
			devices.Put(name, dev)
			if dev4, ok := dev.(net.IPv4Device); ok {
				host.IPv4Host.AddIPv4Device(dev4)
			}
			if dev6, ok := dev.(net.IPv6Device); ok {
				host.IPv6Host.AddIPv6Device(dev6)
			}
		}
	})
}

// typ is the type part of the name; name is the full name
func parseDevName(str string) (typ, name string, err error) {
	parts := strings.Split(str, ":")
	if len(parts) != 2 {
		return "", "", errors.New("device names must be of the form <type>:<suffix>, where <type> is a valid device type")
	}
	return parts[0], str, nil
}

var cmdDev = cli.Command{
	Name:             "dev",
	ShortDescription: "Device-related commands",
	LongDescription:  "Device-related commands.",

	Run: func(c *cli.Command, args []string) {
		if len(args) != 0 {
			c.PrintUsage()
			return
		}
		names := devices.ListNames()
		sort.Strings(names)
		fmt.Println("Devices")
		// TODO(joshlf): Print device's IP addres/subnet
		fmt.Println("Name      MTU       Up     Driver-Specific")
		fmt.Println("==========================================")
		const maxlen = 10
		const maxuplen = 7 // "down" plus a trailing three spaces
		for _, name := range names {
			dev, _ := devices.Get(name)
			mtu := fmt.Sprint(dev.MTU())
			up := "up"
			if !dev.IsUp() {
				up = "down"
			}
			typ, _, _ := parseDevName(name)
			info, err := deviceDrivers[typ].getInfo(dev)
			if err != nil {
				fmt.Printf("get info for %v: %v\n", name, err)
			}
			fmt.Printf("%v%v%v%v\n", name+strings.Repeat(" ", maxlen-len(name)),
				mtu+strings.Repeat(" ", maxlen-len(mtu)),
				up+strings.Repeat(" ", maxuplen-len(up)), info)
		}
	},
}

var cmdDevUp = cli.Command{
	Name:             "up",
	Usage:            "<device>",
	ShortDescription: "Bring a device up",
	LongDescription:  "Bring a device up.",

	Run: func(c *cli.Command, args []string) {
		if len(args) != 1 {
			c.PrintUsage()
			return
		}
		name := args[0]
		dev, ok := devices.Get(name)
		if !ok {
			fmt.Println("no such device")
			return
		}
		err := dev.BringUp()
		if err != nil {
			fmt.Println(err)
		}
	},
}

var cmdDevDown = cli.Command{
	Name:             "down",
	Usage:            "<device>",
	ShortDescription: "Bring a device down",
	LongDescription:  "Bring a device down.",

	Run: func(c *cli.Command, args []string) {
		if len(args) != 1 {
			c.PrintUsage()
			return
		}
		name := args[0]
		dev, ok := devices.Get(name)
		if !ok {
			fmt.Println("no such device")
			return
		}
		err := dev.BringDown()
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	topLevelCommands = append(topLevelCommands, &cmdDev)
	cmdDev.AddSubcommand(&cmdDevUp)
	cmdDev.AddSubcommand(&cmdDevDown)
}
