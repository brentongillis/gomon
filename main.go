// stupid program to launch a go program during development, and watch all
// go and gotmpl files, restarting the program when a file changes
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/brentongillis/go-libs/closer"
	"github.com/fsnotify/fsnotify"
)

const (
	ERROR = iota
	STANDARD
	FMT string = "2006/01/02 15:04:05"
)

type gm struct {
	watcher *fsnotify.Watcher
	cmd     *exec.Cmd
	restart chan struct{}
	args    []string
	dirs    dirs
	msg     string
	c       color
}

func (gm *gm) setDirs() {
	if len(os.Args) < 2 {
		log.Fatal("Please provider a go file to watch")
	}

	// verify the file exists
	_, e := os.Stat(os.Args[1])
	if e != nil {
		log.Fatal(e)
	}

	// walk the path of the target to build a watch list
	root := filepath.Dir(os.Args[1])
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			gm.dirs.Add(path)
		}
		return nil
	})
}

func (gm *gm) setWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	gm.watcher = watcher

	for _, dir := range gm.dirs {
		gm.watcher.Add(dir)
	}

	go func() {
		for event := range gm.watcher.Events {
			switch event.Op {
			// case fsnotify.Create:
			// 	fmt.Println("CREATE", event.Name)
			case fsnotify.Write:
				// fmt.Println("WRITE", event.Name)
				if filepath.Ext(event.Name) == ".go" ||
					filepath.Ext(event.Name) == ".tmpl" {
					gm.restart <- struct{}{}
				}
				// case fsnotify.Remove:
				// 	fmt.Println("REMOVE", event.Name)
				// case fsnotify.Rename:
				// 	fmt.Println("RENAME", event.Name)
				// case fsnotify.Chmod:
				// 	fmt.Println("CHMOD", event.Name)
			}
		}
	}()
}

func (gm *gm) execute() {
	gm.cmd = exec.Command("go", gm.args...)
	gm.cmd.Stdout = os.Stdout
	gm.cmd.Stderr = os.Stderr
	gm.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := gm.cmd.Run(); err != nil {
		gm.eprintf("%s\n", err.Error())
	}
}

// kill the process by killing its pid group, this kills as subprocesses
// as well. This function is registered with the closer
func (gm *gm) killPGroup() int {
	gm.printf("Kill process group\n")
	if e := syscall.Kill(-gm.cmd.Process.Pid, syscall.SIGKILL); e != nil {
		gm.eprintf("%s\n", e.Error())
		return -1
	}
	return 0
}

func (gm *gm) createArgs(args []string) {
	gm.args = append(gm.args, args...)
}

func (gm *gm) eprintf(str string, args ...interface{}) {
	gm.printer(os.Stderr, "e", str, args...)
}

func (gm *gm) printf(str string, args ...interface{}) {
	gm.printer(os.Stdout, "o", str, args...)
}

func (gm *gm) printer(fd *os.File, color, str string, args ...interface{}) {
	fmt.Fprintf(fd, gm.msg, gm.c[color], time.Now().Format(FMT))
	fmt.Fprintf(fd, str, args...)
	fmt.Fprintf(fd, "%s", gm.c["r"])
}

// map of color values to use for gomon printing
// NOTE add ability for user to change these
type color map[string][]byte

// custom string slice for holding directories to watch
// type adds convenience methods for adding and removing
type dirs []string

func (d *dirs) Add(path string) {
	*d = append(*d, path)
}

func (d *dirs) Remove(path string) error {
	i := d.index(path)
	if i == -1 {
		return fmt.Errorf("%s not found in dirs!\n", path)
	}

	(*d)[i] = (*d)[len(*d)-1]
	(*d)[len(*d)-1] = ""
	(*d) = (*d)[:len(*d)-1]

	return nil
}

// returns index of string, used to remove the item in Remove
func (d *dirs) index(path string) int {
	for i := 0; i < len(*d); i++ {
		if (*d)[i] == path {
			return i
		}
	}

	return -1
}

func main() {
	gm := &gm{
		watcher: new(fsnotify.Watcher),
		restart: make(chan struct{}),
		dirs:    make(dirs, 0),
		args:    []string{"run", os.Args[1]},
		msg:     "%s[gomon %s] ",
		c: color{
			"e": []byte{27, 91, 48, 59, 51, 51, 109},
			"o": []byte{27, 91, 48, 59, 51, 54, 109},
			"r": []byte{27, 91, 48, 109},
		},
	}
	// ensure a clean exit
	closer.Register(gm.killPGroup)

	// setup the watcher
	gm.setDirs()
	gm.setWatcher()
	defer gm.watcher.Close()

	// build the args slice to pass into exec.Command
	gm.createArgs(os.Args[2:])

	// loop forever, listening for restart commands
	for {
		go gm.execute()
		<-gm.restart
		gm.printf("Restarting %s\n", os.Args[1])
		gm.killPGroup()

	}
}
