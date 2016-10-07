package main

import (
	"fmt"
	"os"

	"github.com/joshlf/net/example/internal/cli"
	"github.com/spf13/pflag"
)

var (
	// each module is responsible for adding any commands to this
	topLevelCommands []*cli.Command

	// call these after flags.Parse; each module is responsible
	// for adding any functions to this
	postParseFuncs []func()
)

func main() {
	pflag.Parse()
	for _, f := range postParseFuncs {
		f()
	}

	err := cli.RunCLI(topLevelCommands...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
