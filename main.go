// stupid program to launch a go program during development, and watch all
// go and gotmpl files, restarting the program when a file changes
package main

import (
	"github.com/brentongillis/go-libs/closer"
	"github.com/brentongillis/gomon/core"
)

func main() {
	closer.Register(nil)
	core.Monitor()
}
