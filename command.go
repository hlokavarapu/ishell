package ishell

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"sort"
	"text/tabwriter"
)

// Cmd is a shell command handler.
type Cmd struct {
	// Command name.
	Name string
	// Command name aliases.
	Aliases []string
	// Function to execute for the command.
	Func func(c *Context)
	// One liner help message for the command.
	Help string
	// More descriptive help message for the command.
	LongHelp string

	// Completer is custom autocomplete for command.
	// It takes in command arguments and returns
	// autocomplete options.
	// By default all commands get autocomplete of
	// subcommands.
	// A non-nil Completer overrides the default behaviour.
	Completer func(args []string) []string

	// CompleterWithPrefix is custom autocomplete like
	// for Completer, but also provides the prefix
	// already so far to the completion function
	// If both Completer and CompleterWithPrefix are given,
	// CompleterWithPrefix takes precedence
	CompleterWithPrefix func(prefix string, args []string) []string

	// subcommands.
	children map[string]*Cmd

	// optional subcommands
	optionalChildren map[string]*Cmd
}

// AddCmd adds cmd as a subcommand.
func (c *Cmd) AddCmd(cmd *Cmd) {
	if c.children == nil {
		c.children = make(map[string]*Cmd)
	}
	c.children[cmd.Name] = cmd
}

// AddOptionalCmd adds cmd as an optional subcommand
func (c *Cmd) AddOptionalCmd(cmd *Cmd) {
	if c.optionalChildren == nil {
		c.optionalChildren = make(map[string]*Cmd)
	}
	c.optionalChildren[cmd.Name] = cmd
}

// DeleteCmd deletes cmd from subcommands.
func (c *Cmd) DeleteCmd(name string) {
	delete(c.children, name)
}

// Children returns the subcommands of c.
func (c *Cmd) Children() []*Cmd {
	var cmds []*Cmd
	for _, cmd := range c.children {
		cmds = append(cmds, cmd)
	}
	sort.Sort(cmdSorter(cmds))
	return cmds
}

// OptionalChildren returns the subcommands of c.
func (c *Cmd) OptionalChildren() []*Cmd {
	var cmds []*Cmd
	for _, cmd := range c.optionalChildren {
		cmds = append(cmds, cmd)
	}
	sort.Sort(cmdSorter(cmds))
	return cmds
}

func (c *Cmd) hasSubcommand() bool {
	if len(c.children) > 1 {
		return true
	}
	if _, ok := c.children["help"]; !ok {
		return len(c.children) > 0
	}
	return false
}

func (c *Cmd) hasOptionalSubcommands() bool {
	if len(c.OptionalChildren()) > 1 {
		return true
	}
	if _, ok := c.optionalChildren["help"]; !ok {
		return len(c.optionalChildren) > 0
	}
	return false
}

// HelpText returns the computed help of the command and its subcommands.
func (c Cmd) HelpText() string {
	var b bytes.Buffer
	p := func(s ...interface{}) {
		fmt.Fprintln(&b)
		if len(s) > 0 {
			fmt.Fprintln(&b, s...)
		}
	}
	if c.LongHelp != "" {
		p(c.LongHelp)
	} else if c.Help != "" {
		p(c.Help)
	} else if c.Name != "" {
		p(c.Name, "has no help")
	}
	if c.hasSubcommand() {
		p("Commands:")
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, child := range c.Children() {
			fmt.Fprintf(w, "\t%s\t\t\t%s\n", child.Name, child.Help)
		}
		w.Flush()
		p()
	}
	if c.hasOptionalSubcommands() {
		p("Optional Commands:")
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, child := range c.OptionalChildren() {
			fmt.Fprintf(w, "\t%s\t\t\t%s\n", child.Name, child.Help)
		}
		w.Flush()
		p()
	}
	return b.String()
}

// findChildCmd returns the subcommand with matching name or alias.
func (c *Cmd) findChildCmd(name string) *Cmd {
	// find perfect matches first
	if cmd, ok := c.children[name]; ok {
		return cmd
	}

	// find alias matching the name
	for _, cmd := range c.children {
		for _, alias := range cmd.Aliases {
			if alias == name {
				return cmd
			}
		}
	}

	return nil
}

// FindCmd finds the matching Cmd for args.
// It returns the Cmd and the remaining args.
func (c Cmd) FindCmd(args []string) (*Cmd, map[*Cmd]string, []string) {
	var cmd *Cmd
	var remArgs []string
	for i, arg := range args {
		if cmd1 := c.findChildCmd(arg); cmd1 != nil {
			cmd = cmd1
			c = *cmd
			remArgs = args[i+1:]
			continue
		}
	}
	var optCmd *Cmd
	optCmdMap := make(map[*Cmd]string)
	var optArgs string
	for _, arg := range remArgs {
		if cmd1 := c.findOptionalChildCmd(arg); cmd1 != nil {
			if optCmd != nil {
				optCmdMap[optCmd] = optArgs
				optCmd = nil
			}
			optCmd = cmd1
			optArgs = ""
			continue
		} else {
			optArgs = arg
		}
	}
	if optCmd != nil {
		optCmdMap[optCmd] = optArgs
	}
	return cmd, optCmdMap, remArgs
}

// findOptionalChildCmd returns the subcommand with matching name or alias.
func (c *Cmd) findOptionalChildCmd(name string) *Cmd {
	// find perfect matches first
	if cmd, ok := c.optionalChildren[name]; ok {
		return cmd
	}

	// find alias matching the name
	for _, cmd := range c.optionalChildren {
		for _, alias := range cmd.Aliases {
			if alias == name {
				return cmd
			}
		}
	}

	return nil
}

// FindOptionalCmd finds the matching Cmd for args.
// It returns the Cmd and the remaining args.
func (c Cmd) FindOptionalCmd(args []string) (*Cmd, []string) {
	var cmd *Cmd
	for i, arg := range args {
		if cmd1 := c.findOptionalChildCmd(arg); cmd1 != nil {
			cmd = cmd1
			c = *cmd
			continue
		}
		return cmd, args[i:]
	}
	return cmd, nil
}

// IsValid checks if a command's argument value given is valid and complete.
func (c Cmd) IsValid(argValue string) (bool, error) {
	if c.Completer == nil {
		return false, errors.New("Completer must be specified for cmd")
	}
	values := c.Completer(nil)
	valid := false
	for _, completedValue := range values {
		if argValue == completedValue {
			valid = true
		}
	}
	return valid, nil
}

type cmdSorter []*Cmd

func (c cmdSorter) Len() int           { return len(c) }
func (c cmdSorter) Less(i, j int) bool { return c[i].Name < c[j].Name }
func (c cmdSorter) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
