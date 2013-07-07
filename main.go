package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

const (
	testSuffix = "_test.go"
)

var (
	prefix = flag.String("prefix", "flymake_", "The prefix for generated Flymake artifacts.")
	debug  = flag.Bool("debug", false, "Enable extra diagnostic output to determine why errors are occurring.")

	testArguments  = []string{"test", "-c"}
	buildArguments = []string{"build", "-o", "/dev/null"}

	debugLog *log.Logger
)

// buildCommand returns an *exec.Cmd which will build the file or
// package or, in the case of test files, the test binary. If the file
// appears to be part of a package, the entire package (or the test
// binary) is built otherwise only the single file is built. If the
// file is part of a package and has the flymake prefix, the file
// without the prefix is excluded from the build run.
func buildCommand(file string) *exec.Cmd {
	baseName := path.Base(file)
	var ignoreBaseName string

	if strings.HasPrefix(baseName, *prefix) {
		ignoreBaseName = baseName[len(*prefix):]
	}

	isTest := false
	var goArguments []string

	if strings.HasSuffix(file, testSuffix) {
		isTest = true
		// shame there is no '-o' option
		goArguments = append(goArguments, testArguments...)
	} else {
		goArguments = append(goArguments, buildArguments...)
	}

	pkg, err := build.ImportDir(path.Dir(file), build.AllowBinary)

	if err != nil {
		goArguments = append(goArguments, file)
	} else {
		var files []string
		files = append(files, pkg.GoFiles...)
		files = append(files, pkg.CgoFiles...)
		if isTest {
			files = append(files, pkg.TestGoFiles...)
			files = append(files, pkg.XTestGoFiles...)
		}

		for _, f := range files {
			if f == ignoreBaseName {
				continue
			}
			goArguments = append(goArguments, f)
		}
	}

	debugLog.Println("go build arguments:", goArguments)

	return exec.Command("go", goArguments...)
}

// vetCommand returns an *exec.Cmd which will run go vet on the given
// file.
func vetCommand(file string) *exec.Cmd {
	return exec.Command("go", "vet", file)
}

func init() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("%s some_file.go", path.Base(os.Args[0]))
	}

	var writer io.Writer

	if *debug {
		writer = os.Stderr
	} else {
		writer = ioutil.Discard
	}

	debugLog = log.New(writer, "goflymake:", 0)
	debugLog.Println("Arguments:", os.Args)
	debugLog.Println("PATH:", os.Getenv("PATH"))
	debugLog.Println("GOPATH", os.Getenv("GOPATH"))
	debugLog.Println("GOROOT", os.Getenv("GOROOT"))
}

// printCmdOutput start the given command and prints each line the
// command produces on standard output and standard error.
func printCmdOutput(cmd exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	printPipe := func(pipe io.ReadCloser) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			debugLog.Println(scanner.Text())
			fmt.Println(scanner.Text())
		}
	}

	wg.Add(2)
	go printPipe(stdout)
	go printPipe(stderr)

	wg.Wait()
}

func main() {
	testFile := flag.Args()[0]
	var wg sync.WaitGroup

	runCmd := func(cmd *exec.Cmd) {
		defer wg.Done()
		printCmdOutput(*cmd)
	}

	cmds := [...]*exec.Cmd{buildCommand(testFile), vetCommand(testFile)}

	wg.Add(len(cmds))
	for _, cmd := range cmds {
		go runCmd(cmd)
	}

	wg.Wait()
}
