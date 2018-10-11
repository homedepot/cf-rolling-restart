package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpinner_Next(t *testing.T) {
	var b bytes.Buffer
	spinner := NewSpinner(&b)

	spinner.Next()
	require.Equal(t, "\r/", b.String(), "Output should cycle correctly.")
	b.Reset()

	spinner.Next()
	require.Equal(t, "\r-", b.String(), "Output should cycle correctly.")
	b.Reset()

	spinner.Next()
	require.Equal(t, "\r\\", b.String(), "Output should cycle correctly.")
	b.Reset()

	spinner.Next()
	require.Equal(t, "\r|", b.String(), "Output should cycle correctly.")
}

func TestSpinner_Done(t *testing.T) {
	var b bytes.Buffer
	spinner := NewSpinner(&b)

	spinner.Next()
	require.Equal(t, "\r/", b.String(), "Output should cycle correctly.")
	b.Reset()

	spinner.Done()
	require.Equal(t, "\x1b[32;1m\rOK\n\x1b[0m", b.String(), "Done message should display over spinner.")

}
