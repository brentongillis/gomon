package core

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/fsnotify/fsnotify"
)

const (
	ERROR = iota
	STANDARD
)

type Channels struct {
	Exit    chan struct{}
	Restart chan struct{}
	IO      chan Stdio
}

func watchDir(channels Channels) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for event := range watcher.Events {
			if event.Op&fsnotify.Write == fsnotify.Write {
				if strings.Contains(event.Name, ".go") ||
					strings.Contains(event.Name, ".tmpl") {
					channels.Exit <- struct{}{}
					channels.Restart <- struct{}{}
				}
			}
		}
	}()

	if err := watcher.Add("."); err != nil {
		log.Fatal(err)
	}
	return watcher
}

func Monitor() {
	channels := Channels{
		Exit:    make(chan struct{}),
		Restart: make(chan struct{}),
		IO:      make(chan Stdio),
	}
	w := watchDir(channels)
	defer w.Close()

	args := createArgs(os.Args[2:])
	go stdioScanner(channels.IO)
	go stdioScanner(channels.IO)

	for {
		cmd := execute(channels, args)
		go func() {
			<-channels.Exit
			if e := cmd.Process.Kill(); e != nil {
				fmt.Fprintln(os.Stderr, "CMD ERROR", e)
			}
		}()

		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "Program %s has exited %s\n", os.Args[1], err.Error())
		}

		<-channels.Restart
	}

	// wait forever
	<-make(chan struct{})
}

func execute(channels Channels, args string) *exec.Cmd {
	cmd := exec.Command("go", "run", os.Args[1], args)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	channels.IO <- Stdio{STANDARD, stdout}
	channels.IO <- Stdio{ERROR, stderr}

	return cmd
}

func createArgs(args []string) string {
	var str string
	for _, f := range args {
		str += f + " "
	}
	return str
}

type Stdio struct {
	Color      int
	ReadCloser io.ReadCloser
}

func stdioScanner(stdio chan Stdio) {
	for {
		std := <-stdio
		leader := "\x1B["
		switch std.Color {
		case STANDARD:
			leader += "0;36m"
		case ERROR:
			leader += "0;33m"
		}
		scanner := bufio.NewScanner(std.ReadCloser)
		for scanner.Scan() {
			fmt.Printf("%s%s\x1B[0m\n", leader, scanner.Text())
		}
	}
}
