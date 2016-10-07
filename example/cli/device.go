package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
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
			fmt.Fprintln(os.Stderr, "could not open device file: ", err)
			os.Exit(2)
		}
		pairs, err := internal.ParsePairs(devfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not parse device file: ", err)
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
        host.IPv4.AddDevice(dev4)
      }
      if dev6, ok := dev.(net.IPv6Device); ok {
        host.IPv6.AddDevice(dev6)
      }
		}
	})
}

// typ is the type part of the name; name is the full name
func parseDevName(str string) (typ, name string, err error) {
	var i int
	for ; i < len(str); i++ {
		if str[i] < 'a' || str[i] > 'z' {
			break
		}
	}

	const errstr = "device names must be of the form <type><num>, where <type> is a valid device type"
	if i == 0 || i == len(str) {
		// no leading a-z characters or nothing but a-z characters
		return "", "", errors.New(errstr)
	}
	typ = str[:i]

	// TODO(joshlf): Currently, dev0 and dev00 (or dev1 and dev01) are different
	_, err = strconv.ParseUint(str[i:], 10, 64)
	if err != nil {
		// must not be a non-negative number
		return "", "", errors.New(errstr)
	}
	return typ, str, nil
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
		fmt.Println("=======")
		for _, name := range names {
			dev, _ := devices.Get(name)
			fmt.Printf("%v %v\n", name, dev.IsUp())
		}
	},
}

func init() {
	topLevelCommands = append(topLevelCommands, &cmdDev)
}
