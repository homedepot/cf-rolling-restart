package main

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

var states = map[string]string{
	"|":  "/",
	"/":  "-",
	"-":  "\\",
	"\\": "|"}

// Provides state for a basic command line spinner.
type Spinner struct {
	writer io.Writer
	state  string
}

// Returns a new Spinner object.
func NewSpinner(writer io.Writer) *Spinner {
	return &Spinner{writer, "|"}
}

// Progresses the spinner to the next state.
func (spinner *Spinner) Next() {
	spinner.state = states[spinner.state]
	fmt.Fprintf(spinner.writer, "\r%s", spinner.state)
}

// Outputs a green OK in place of the spinner when called.
func (spinner *Spinner) Done() {
	color.New(color.FgGreen, color.Bold).Fprint(spinner.writer, "\rOK\n")
}
