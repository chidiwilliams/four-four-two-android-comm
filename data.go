package main

import "fmt"

type command struct {
	Action string   `json:"action"`
	Args   []string `json:"args,omitempty"`
}

func (c *command) String() string {
	return fmt.Sprintf("{%v %v}", c.Action, c.Args)
}
