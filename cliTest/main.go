package main

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(initialModel())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

type errMsg error
type model struct {
	textInput textinput.Model
	err       error
}
type flag struct {
	name, shorthand, desc string
}

func (f flag) matchName(s string) bool {
	reg, _ := regexp.Compile(fmt.Sprintf("^%s.*", s))
	return reg.MatchString("--" + f.name)
}
func (f flag) matchShorthand(s string) bool {
	reg, _ := regexp.Compile(fmt.Sprintf("^%s.*", s))
	return reg.MatchString("-" + f.shorthand)
}

type command struct {
	name        string
	subCommands map[string]command
	flags       []flag
	desc        string
}

func (c command) match(s, p string) []string {
	comms := strings.Split(s, " ")
	reg, _ := regexp.Compile(fmt.Sprintf("^%s.*", s))
	out := []string{}
	for key, val := range c.subCommands {
		if comms[0] == key {
			if len(comms) > 1 {
				out = append(out, val.match(strings.Join(comms[1:], " "), p+c.name+" ")...)
			}
		} else if reg.MatchString(key) {
			out = append(out, p+c.name+" "+key)
		}
	}
	if len(out) == 0 {
		out = append(out, c.findFlag(comms[len(comms)-1], p+c.name)...)
	}
	return out

}
func (c command) findFlag(s, p string) []string {
	out := []string{}
	if len(c.flags) == 0 {
		return out
	}
	for _, f := range c.flags {
		if f.matchName(s) {
			out = append(out, p+" --"+f.name)
		}
		if f.matchShorthand(s) {
			out = append(out, p+" -"+f.shorthand)
		}
	}
	return out
}

var test command = command{
	name: "", subCommands: map[string]command{
		"opa": {
			name: "opa", subCommands: map[string]command{
				"bench": {
					name: "bench", flags: []flag{
						{name: "--benchmem"},
						{shorthand: "-b", name: "--bundle"},
						{name: "--count"},
						{shorthand: "-d", name: "--data"},
						{name: "--fail"},
						{shorthand: "-h", name: "--help"},
						{name: "--ignore"},
						{name: "--import"},
						{shorthand: "-i", name: "--input"},
						{name: "--metrics"},
						{name: "--package"},
						{shorthand: "-p", name: "--partial"},
						{shorthand: "-s", name: "--schema"},
						{name: "--stdin"},
						{shorthand: "-I", name: "--stdin-input"},
						{shorthand: "-t", name: "--target"},
						{shorthand: "-u", name: "--unknowns"},
					},
				},
				"build": {
					name: "build", flags: []flag{
						{shorthand: "b", name: "bundle"},
						{name: "capabilities"},
						{name: "claimsfile"},
						{name: "debug"},
						{shorthand: "e", name: "entrypoint"},
						{name: "excludefilesverify"},
						{shorthand: "h", name: "help"},
						{name: "ignore"},
						{shorthand: "O", name: "optimize"},
						{shorthand: "o", name: "output"},
						{shorthand: "r", name: "revision"},
						{name: "scope"},
						{name: "signingalg"},
						{name: "signingkey"},
						{name: "signingplugin"},
						{shorthand: "t", name: "target"},
						{name: "verificationkey"},
						{name: "verificationkeyid"},
					},
				},
				"check": {
					name: "check", flags: []flag{
						{shorthand: "b", name: "bundle"},
						{shorthand: "d", name: "data"},
						{shorthand: "f", name: "format"},
						{shorthand: "h", name: "help"},
						{name: "ignore"},
					},
				},
				"deps": {
					name: "deps", flags: []flag{
						{shorthand: "b", name: "bundle"},
						{shorthand: "d", name: "data"},
						{shorthand: "f", name: "format"},
						{shorthand: "h", name: "help"},
						{name: "ignore"},
					},
				},
				"eval": {
					name: "eval", flags: []flag{
						{shorthand: "b", name: "bundle"},
						{name: "coverage"},
						{shorthand: "d", name: "data"},
						{name: "disableindexing"},
						{name: "disableinlining"},
						{name: "explain"},
						{name: "fail"},
						{name: "faildefined"},
						{shorthand: "f", name: "format"},
						{shorthand: "h", name: "help"},
						{name: "ignore"},
						{name: "import"},
						{shorthand: "i", name: "input"},
						{name: "instrument"},
						{name: "metrics"},
						{name: "package"},
						{shorthand: "p", name: "partial"},
						{name: "prettylimit"},
						{name: "profile"},
						{name: "profilelimit"},
						{name: "profilesort"},
						{shorthand: "s", name: "schema"},
						{name: "shallowinlining"},
						{name: "stdin"},
						{shorthand: "I", name: "stdininput"},
						{name: "strictbuiltinerrors"},
						{shorthand: "t", name: "target"},
						{shorthand: "u", name: "unknowns"},
					},
				},
				"fmt": {
					name: "fmt", flags: []flag{
						{shorthand: "d", name: "diff"},
						{name: "fail"},
						{shorthand: "h", name: "help"},
						{shorthand: "l", name: "list"},
						{shorthand: "w", name: "write"},
					},
				},
				"help": {
					name: "help", flags: []flag{
						{shorthand: "h", name: "help"},
					},
				},
				"parse": {
					name: "parse", flags: []flag{
						{shorthand: "f", name: "format"},
						{shorthand: "h", name: "help"},
					},
				},
				"run": {
					name: "run", flags: []flag{
						{shorthand: "a", name: "addr"},
						{name: "authentication"},
						{name: "authorization"},
						{shorthand: "b", name: "bundle"},
						{shorthand: "c", name: "configfile"},
						{name: "diagnosticaddr"},
						{name: "excludefilesverify"},
						{shorthand: "f", name: "format"},
						{name: "h2c"},
						{shorthand: "h", name: "help"},
						{shorthand: "H", name: "history"},
						{name: "ignore"},
						{name: "logformat"},
						{shorthand: "l", name: "loglevel"},
						{shorthand: "m", name: "maxerrors"},
						{name: "pprof"},
						{name: "readytimeout"},
						{name: "scope"},
						{shorthand: "s", name: "server"},
						{name: "set"},
						{name: "setfile"},
						{name: "shutdowngraceperiod"},
						{name: "shutdownwaitperiod"},
						{name: "signingalg"},
						{name: "skipverify"},
						{name: "skipversioncheck"},
						{name: "tlscacertfile"},
						{name: "tlscertfile"},
						{name: "tlsprivatekeyfile"},
						{name: "verificationkey"},
						{name: "verificationkeyid"},
						{shorthand: "w", name: "watch"},
					},
				},
				"server": {
					name: "server", flags: []flag{},
				},
				"repl": {
					name: "repl", flags: []flag{},
				},
				"sign": {
					name: "sign", flags: []flag{
						{shorthand: "b", name: "bundle"},
						{name: "claimsfile"},
						{shorthand: "h", name: "help"},
						{shorthand: "o", name: "outputfilepath"},
						{name: "signingalg"},
						{name: "signingkey"},
						{name: "signingplugin"},
					},
				},
				"test": {
					name: "test", flags: []flag{
						{name: "bench"},
						{name: "benchmem"},
						{shorthand: "b", name: "bundle"},
						{name: "count"},
						{shorthand: "c", name: "coverage"},
						{name: "explain"},
						{shorthand: "f", name: "format"},
						{shorthand: "h", name: "help"},
						{name: "ignore"},
						{shorthand: "m", name: "maxerrors"},
						{shorthand: "r", name: "run"},
						{shorthand: "t", name: "target"},
						{name: "threshold"},
						{name: "timeout"},
						{shorthand: "v", name: "verbose"},
					},
				},
				"version": {
					name: "version", flags: []flag{
						{shorthand: "c", name: "check"},
						{shorthand: "h", name: "help"},
					},
				},
			},
		},
	},
}
var names []string = []string{"Pikachu", "Ponyta", "Bulbasaur", "Charmander", "Squirtel", "Agron", "Eevee", "Mewtwo", "Raichu", "Venusaur"}
var commands cList = cList{[]string{}, 0}

type cList struct {
	commands []string
	index    int
}

func (c *cList) get() string { return c.commands[c.index] }
func (c *cList) getPrev() (string, error) {
	if c.index <= 0 {
		return "", errors.New("")
	}
	c.index--
	return c.commands[c.index], nil
}
func (c *cList) getNext() (string, error) {
	if c.index >= len(c.commands)-1 {
		return "", errors.New("")
	}
	c.index++
	return c.commands[c.index], nil
}
func (c *cList) add(s string) {
	if len(c.commands) == 0 {
		c.index++
		c.commands = append(c.commands, s)
	} else {
		c.index = len(c.commands) - 1
		if s != c.commands[c.index] {
			c.commands = append(c.commands, s)
		}
	}
	c.index = len(c.commands)
}
func matchThingie(i string) []string {
	out := []string{}
	reg, _ := regexp.Compile(fmt.Sprintf("^%s.*", i))
	for _, name := range names {
		if reg.MatchString(name) {
			out = append(out, name)
		}
	}
	return out
}
func initialModel() model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	fmt.Println("REGO test")
	return model{
		textInput: ti,
		err:       nil,
	}
}
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			str := test.match(m.textInput.Value(), "")
			if len(str) == 1 {
				m.textInput.SetValue(str[0][1:])
				m.textInput.CursorEnd()
			} else {
				for i := range str {
					str[i] = strings.Split(str[i], " ")[len(strings.Split(str[i], " "))-1]
				}
				fmt.Print("\n ", str, "\n")
			}
		case tea.KeyEnter:
			fmt.Print("\n")
			commands.add(m.textInput.Value())
			m.textInput.SetValue("")
		case tea.KeyUp:
			prev, err := commands.getPrev()
			if err == nil {
				m.textInput.SetValue(prev)
			}
		case tea.KeyDown:
			next, err := commands.getNext()
			if err == nil {
				m.textInput.SetValue(next)
			}
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.textInput.View()
}
