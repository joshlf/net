package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/juju/errors"
)

type Command struct {
	Name             string
	Usage            string // single-line usage string, not including command name
	ShortDescription string // single-line summary
	LongDescription  string // multi-line description

	// Run is the function to call when the command is executed.
	Run func(cmd *Command, args []string)

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

func (c *Command) Execute(args string) {
	fields := strings.Fields(args)
	var subcmd *Command
	var ok bool
	if len(fields) != 0 {
		// if len(fields) == 0, then there's definitely no subcommand
		if fields[0] == "-h" || fields[0] == "--help" {
			c.PrintUsage()
			return
		}
		subcmd, ok = c.subcommands[fields[0]]
	}

	if !ok {
		if c.Run == nil {
			if len(c.subcommands) == 0 {
				panic("command has no run function or subcommands")
			}
			c.PrintUsage()
			return
		}
		c.Run(c, fields)
		return
	}
	subcmd.Execute(strings.Join(fields[1:], " "))
}

// Print long-form usage.
func (c *Command) PrintUsage() {
	if c.Usage != "" {
		fmt.Printf("Usage: %v %v\n", c.Name, c.Usage)
		fmt.Println()
	}
	fmt.Println(c.LongDescription)

	if len(c.subcommands) > 0 {
		// add a space between the description and the subcommands
		fmt.Println()

		fmt.Println("Available subcommands:")
		var cmds []*Command
		for _, cc := range c.subcommands {
			cmds = append(cmds, cc)
		}
		printAvailableCommands(cmds...)
	}
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
	c.Execute(strings.Join(fields[1:], " "))
	return nil
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
	longestName += 3 // at least 3 spaces between the name and the summary

	for _, c := range cmds {
		left := longestName - len(c.Name)
		fmt.Printf("%v%v%v\n", c.Name, strings.Repeat(" ", left), c.ShortDescription)
	}
}

// RunCLI runs an interactive command-line interface, reading from os.Stdin
// and writing to os.Stdout
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
