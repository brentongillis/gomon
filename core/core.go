package core

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	ERROR = iota
	STANDARD
	FMT string = "2006/01/02 15:04:05"
)

type Stdio struct {
	Color      int
	ReadCloser io.ReadCloser
}

type GM struct {
	Watcher *fsnotify.Watcher
	Exit    chan struct{}
	Restart chan struct{}
	IO      chan Stdio
	Args    string
	msg     string
	c       color
}

type color struct {
	e [7]byte
	o [7]byte
	r [4]byte
}

func newGM() *GM {
	return &GM{
		Watcher: new(fsnotify.Watcher),
		Exit:    make(chan struct{}),
		Restart: make(chan struct{}),
		IO:      make(chan Stdio),
		Args:    "",
		msg:     "%s[gomon %s] ",
		c: color{
			e: [7]byte{27, 91, 48, 59, 51, 51, 109},
			o: [7]byte{27, 91, 48, 59, 51, 54, 109},
			r: [4]byte{27, 91, 48, 109},
		},
	}
}

func (gm *GM) setWatcher(path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	gm.Watcher = watcher

	go func() {
		for event := range watcher.Events {
			if event.Op&fsnotify.Write == fsnotify.Write {
				if strings.Contains(event.Name, ".go") ||
					strings.Contains(event.Name, ".tmpl") {
					gm.Exit <- struct{}{}
					gm.Restart <- struct{}{}
				}
			}
		}
	}()

	if err := watcher.Add("."); err != nil {
		log.Fatal(err)
	}
}

func (gm *GM) stdioScanner() {
	for {
		std := <-gm.IO
		scanner := bufio.NewScanner(std.ReadCloser)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}
}

func (gm *GM) execute() *exec.Cmd {
	cmd := exec.Command("go", "run", os.Args[1], gm.Args)
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

	gm.IO <- Stdio{STANDARD, stdout}
	gm.IO <- Stdio{ERROR, stderr}

	return cmd
}

func (gm *GM) createArgs(args []string) {
	for _, f := range args {
		gm.Args += f + " "
	}
}

func (gm *GM) eprintf(fd *os.File, str string, args ...interface{}) {
	fmt.Fprintf(fd, gm.msg, string(gm.c.e[:7]), time.Now().Format(FMT))
	fmt.Fprintf(fd, str, args...)
	fmt.Fprintf(fd, string(gm.c.r[:4]))
}

func Monitor() {
	gm := newGM()
	gm.setWatcher(".")
	defer gm.Watcher.Close()

	gm.createArgs(os.Args[2:])
	go gm.stdioScanner()
	go gm.stdioScanner()

	for {
		cmd := gm.execute()
		go func() {
			<-gm.Exit
			if e := cmd.Process.Kill(); e != nil {
				gm.eprintf(os.Stderr, "Kill process error :: %s\n", e.Error())
			}
		}()

		if err := cmd.Wait(); err != nil {
			gm.eprintf(os.Stderr, "Program %s has exited %s\n",
				os.Args[1], err.Error(),
			)
		}

		<-gm.Restart
	}

	// wait forever
	<-make(chan struct{})
}
