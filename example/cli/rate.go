package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/joshlf/net"
	"github.com/joshlf/net/example/internal/cli"
	"github.com/joshlf/rate"
)

var cmdIPRead = cli.Command{
	Name:             "read",
	Usage:            "<file> <destination> <protocol number> [--ttl <ttl>]",
	ShortDescription: "Copy a file over an IP connection",
	LongDescription: `Copy the given file by sending its body in successive IP packets
with the given protocol number and, optionally, TTL.
The size of the IP packets will be automatically adjusted
based on the MTU of the device.`,

	Run: func(cmd *cli.Command, args []string) {
		if len(args) != 3 && (len(args) != 5 || args[3] != "--ttl") {
			cmd.PrintUsage()
			return
		}

		dst, err := net.ParseIP(args[1])
		if err != nil {
			fmt.Println("could not parse destination IP:", err)
			return
		}
		proto, err := strconv.ParseUint(args[2], 10, 8)
		if err != nil {
			fmt.Println("could not parse protocol number:", err)
			return
		}
		var doTTL bool
		var ttl uint64
		if len(args) == 5 {
			doTTL = true
			ttl, err = strconv.ParseUint(args[3], 10, 8)
			if err != nil {
				fmt.Println("could not parse ttl:", err)
				return
			}
		}

		f, err := os.Open(args[0])
		defer f.Close()
		if err != nil {
			fmt.Println("could not open file:", err)
			return
		}

		// start the buffer large and shrink until we don't get MTU errors
		buf := make([]byte, 32768)
		for err == nil {
			var n int
			n, err = f.Read(buf)
			if n > 0 {
				if doTTL {
					_, err = host.WriteToTTL(buf[:n], dst, net.IPProtocol(proto), uint8(ttl))
				} else {
					_, err = host.WriteTo(buf[:n], dst, net.IPProtocol(proto))
				}
				if net.IsMTU(err) && len(buf) > 0 {
					// check len(buf) > 0 in case we have a pathological device
					buf = buf[:len(buf)/2]
					err = nil
					continue
				}
				if err != nil {
					fmt.Println("could not write IP packet:", err)
					return
				}
			}
		}
		if err != io.EOF {
			fmt.Println("could not read file:", err)
		}
	},
}

var cmdIPRateMonitor = cli.Command{
	Name:             "rate-monitor",
	Usage:            "<protocol number> [-p | --progress] [--B | --kB | --KiB | --MB | --MiB | --GB | --GiB]",
	ShortDescription: "Monitor the rate of incoming IP packets",
	LongDescription: `Monitor the rate of incoming IP packets with the given protocol number.
The flags control the units in which the rate is displayed,
and whether or not progress is recorded. Press enter to stop.`,

	Run: func(cmd *cli.Command, args []string) {
		if len(args) < 2 || len(args) > 3 {
			cmd.PrintUsage()
			return
		}
		unit, size, progress, ok := parseRateFlags(args[1:])
		if !ok {
			cmd.PrintUsage()
			return
		}

		proto, err := strconv.ParseUint(args[0], 10, 8)
		if err != nil {
			fmt.Println("could not parse protocol number:", err)
			return
		}

		r := rate.MakeMonitorFunc(0, rateFn(size, unit, progress))
		// r := rate.MakeMonitorReaderFunc(os.Stdin, 0, rateFn(size, unit, progress))
		defer func() { r.Close(); fmt.Println() }()
		host.RegisterCallback(func(b []byte, src, dst net.IP) {
			r.Add(uint64(len(b)))
		}, net.IPProtocol(proto))
		bufio.NewScanner(os.Stdin).Scan()
	},
}

func init() {
	cmdIP.AddSubcommand(&cmdIPRateMonitor)
	cmdIP.AddSubcommand(&cmdIPRead)
}

func parseRateFlags(args []string) (unit string, size int, progress, ok bool) {
	units := map[string]int{
		"--B":   1,
		"--kB":  1000,
		"--KiB": 1024,
		"--MB":  1000 * 1000,
		"--MiB": 1024 * 1024,
		"--GB":  1000 * 1000 * 1000,
		"--GiB": 1024 * 1024 * 1024,
	}

	switch {
	case len(args) > 2:
		return "", 0, false, false
	case len(args) == 2 && args[0] != "-p" && args[0] != "--progress":
		return "", 0, false, false
	case len(args) == 2:
		progress = true
		unit = args[1]
	default:
		unit = args[0]
	}
	size, ok = units[unit]
	if !ok {
		return "", 0, false, false
	}
	// remove the leading -- from the unit
	return unit[2:], size, progress, true
}

func rateFn(size int, unit string, progress bool) func(r rate.Rate) {
	return func(r rate.Rate) {
		fmt.Fprintf(os.Stderr, "\r%8.4f %s/s\033[K", r.Rate/float64(size), unit)

		if progress {
			sizeTmp, unitTmp := 0, ""

			// If the rate is displayed in an SI unit,
			// then display the progress in SI units,
			// and likewise for IEC units.
			if strings.Contains(unit, "i") || unit == "B" {
				switch {
				case r.Total < 1024:
					sizeTmp, unitTmp = 1, "B"
				case r.Total < 1024*1024:
					sizeTmp, unitTmp = 1024, "KiB"
				case r.Total < 1024*1024*1024:
					sizeTmp, unitTmp = 1024*1024, "MiB"
				default:
					sizeTmp, unitTmp = 1024*1024*1024, "GiB"
				}
			} else {
				switch {
				case r.Total < 1000:
					sizeTmp, unitTmp = 1, "B"
				case r.Total < 1000*1000:
					sizeTmp, unitTmp = 1000, "kB"
				case r.Total < 1000*1000*1000:
					sizeTmp, unitTmp = 1000*1000, "MB"
				default:
					sizeTmp, unitTmp = 1000*1000*1000, "GB"
				}
			}
			fmt.Fprintf(os.Stderr, " (%.4f %s total)\033[K", float64(r.Total)/float64(sizeTmp), unitTmp)
		}
	}
}
