package main

import (
	"fmt"
	"github.com/fatih/color"
	"io"
)

var states = map[string]string{
	"|": "/",
	"/": "-",
	"-": "\\",
	"\\": "|"}

type Spinner struct {
	writer		io.Writer
	state 		string
}

func NewSpinner(writer io.Writer) *Spinner {
	return &Spinner{writer, "|"}
}

func (spinner *Spinner) Next() {
	spinner.state = states[spinner.state]
	fmt.Fprintf(spinner.writer, "\r%s", spinner.state)
}

func (spinner *Spinner) Done() {
	color.New(color.FgGreen, color.Bold).Fprint(spinner.writer, "\rOK\n")
}