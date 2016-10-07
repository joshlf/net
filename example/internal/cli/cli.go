package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

type Command struct {
	Name       string
	ShortUsage string // single-line usage summary
	LongUsage  string // multi-line usage

	// Run is the function to call when the command is executed.
	// Errors should only be returned that relate to execution
	// of the CLI framework such as failing to write to stdout.
	// Other non-CLI-related errors such as network errors should
	// be reported directly to the user as appropriate.
	Run func(cmd *Command, args []string) error

	subcommands map[string]*Command
}

func (c *Command) validate() {
	switch {
	case c.Name == "":
		panic("empty command name")
	case c.Name == "help":
		panic("reserved command name: help")
	}
}

func (c *Command) AddSubcommand(cmds ...*Command) {
	if c.subcommands == nil {
		c.subcommands = make(map[string]*Command)
	}
	for _, cc := range cmds {
		cc.validate()
		if cc == c {
			panic("Command can't be a child of itself")
		}
		c.subcommands[cc.Name] = cc
	}
}

func (c *Command) Execute(args string) error {
	fields := strings.Fields(args)
	var subcmd *Command
	var ok bool
	if len(fields) != 0 {
		// if len(fields) == 0, then there's definitely no subcommand
		subcmd, ok = c.subcommands[fields[0]]
	}

	if !ok {
		if c.Run == nil {
			if len(c.subcommands) == 0 {
				panic("command has no run function or subcommands")
			}
			fmt.Printf("Available subcommands for %v:\n", c.Name)
			var cmds []*Command
			for _, cc := range c.subcommands {
				cmds = append(cmds, cc)
			}
			printAvailableCommands(cmds...)
			return nil
		}
		return c.Run(c, nil)
	}
	return subcmd.Execute(strings.Join(fields[1:], " "))
}

func ExecuteCommands(args string, cmds ...*Command) error {
	cmdmap := make(map[string]*Command)
	for _, c := range cmds {
		c.validate()
		cmdmap[c.Name] = c
	}

	fields := strings.Fields(args)
	if len(fields) == 0 {
		return nil
	}
	if fields[0] == "help" {
		fmt.Println("Available commands:")
		printAvailableCommands(cmds...)
		return nil
	}
	c, ok := cmdmap[fields[0]]
	if !ok {
		return noCommandErr(fields[0])
	}
	return c.Execute(strings.Join(fields[1:], " "))
}

func printAvailableCommands(cmds ...*Command) {
	// deep copy so that when we sort them, we don't modify the original
	cmds = append([]*Command(nil), cmds...)
	sort.Sort(sortableCommands(cmds))

	var longestName int
	for _, c := range cmds {
		if len(c.Name) > longestName {
			longestName = len(c.Name)
		}
	}

	for _, c := range cmds {
		left := longestName - len(c.Name)
		fmtstr := "%" + strconv.Itoa(left) + "v"
		fmt.Printf(fmtstr+" - %v\n", c.Name, c.ShortUsage)
	}
}

// RunCLI runs an interactive command-line interface, reading from in and writing
// to out. It is assumed that the human interface which provides in and out
// behave like a normal terminal, with both typed (input) and output characters
// being printed to the same displaly buffer. It is the responsibility of any
// command's Run function to terminate all output with a newline so that output
// is properly pretty-printed.
func RunCLI(cmds ...*Command) (err error) {
	s := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for s.Scan() {
		err = ExecuteCommands(s.Text(), cmds...)
		switch {
		case IsNoCommand(err):
			fmt.Println(err)
			fmt.Println("To list available commands, type 'help'.")
		case err != nil:
			return errors.Annotate(err, "run CLI")
		}
		fmt.Print("> ")
	}
	if err = s.Err(); err != nil {
		return errors.Annotate(err, "run CLI")
	}
	fmt.Println()
	return nil
}

type sortableCommands []*Command

func (s sortableCommands) Len() int           { return len(s) }
func (s sortableCommands) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s sortableCommands) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type noCommandErr string

func (err noCommandErr) Error() string {
	return "no such command: " + string(err)
}

// IsNoCommand returns true if err results from a command not existing.
func IsNoCommand(err error) bool {
	_, ok := errors.Cause(err).(noCommandErr)
	return ok
}
