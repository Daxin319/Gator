package main

import (
	"fmt"
	"os"

	config "github.com/Daxin319/Gator/internal/config"
)

func main() {
	configFile := config.Read()

	currentState := state{
		&configFile,
	}

	commands := commands{
		make(map[string]func(*state, command) error),
	}

	commands.register("login", handlerLogins)

	args := os.Args

	if len(args) < 2 {
		err := fmt.Errorf("not enough arguments")
		fmt.Println(err)
		os.Exit(1)
	}

	command := command{
		args[1],
		args[2:],
	}

	commands.run(&currentState, command)
}

type state struct {
	config *config.Config
}

type command struct {
	name      string
	arguments []string
}

type commands struct {
	validCommands map[string]func(*state, command) error
}

func handlerLogins(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		err := fmt.Errorf("only one username expected")
		fmt.Println(err)
		os.Exit(1)
	}
	s.config.SetUser(cmd.arguments[0])
	fmt.Printf("Username set to %s\n", cmd.arguments[0])
	return nil
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.validCommands[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	err := c.validCommands[cmd.name](s, cmd)
	if err != nil {
		return fmt.Errorf("unknown command")
	}

	return nil
}
