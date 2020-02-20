package main

import "fmt"

type Command struct {
	Action string   `json:"action"`
	Args   []string `json:"args,omitempty"`
}

func (c *Command) String() string {
	return fmt.Sprintf("{%v %v}", c.Action, c.Args)
}
