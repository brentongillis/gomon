// stupid program to launch a go program during development, and watch all
// go and gotmpl files, restarting the program when a file changes
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

func main() {
	closer()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	var cmd *exec.Cmd

	restart := make(chan bool)
	done := make(chan bool)
	go func() {
		for event := range watcher.Events {
			if event.Op&fsnotify.Write == fsnotify.Write {
				if strings.Contains(event.Name, ".go") ||
					strings.Contains(event.Name, ".tmpl") {
					if e := cmd.Process.Kill(); e != nil {
						// log.Println(e)
					}
					restart <- true
				}
			}
		}
	}()

	if err := watcher.Add("."); err != nil {
		log.Fatal(err)
	}

	for {
		cmd = exec.Command("go", "run", os.Args[1])
		std, _ := cmd.StdoutPipe()
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		var buffer bytes.Buffer
		buffer.ReadFrom(std)
		log.Printf("%s", buffer.String())

		if err := cmd.Wait(); err != nil {
			// fmt.Fprintln(os.Stderr, err)
		}

		<-restart
	}
	<-done
}

func closer() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		os.Exit(1)
	}()
}
