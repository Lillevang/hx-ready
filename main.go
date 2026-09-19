// hx-ready answers: "I want to use Helix for language X. What am I missing,
// and how do I install it?"
package main

import (
	"os"

	"github.com/Lillevang/hx-ready/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
