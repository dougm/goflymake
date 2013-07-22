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
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type (
	transformer interface {
		transform(input io.Reader, output io.Writer)
	}
	echoTransform    struct{}
	warningTransform struct{}

	diffTransform struct {
		name string
	}

	flymakeCommand struct {
		cmd         *exec.Cmd
		stdoutXform transformer
		stderrXform transformer
	}
)

const (
	testSuffix = "_test.go"
)

var (
	prefix = flag.String("prefix", "flymake_", "The prefix for generated Flymake artifacts.")
	debug  = flag.Bool("debug", false, "Enable extra diagnostic output to determine why errors are occurring.")

	lineNumRegex  = regexp.MustCompile(":[0-9]+:")
	diffHunkRegex = regexp.MustCompile("^@@ -([0-9]+)")

	testFile string

	testArguments  = []string{"test", "-c"}
	buildArguments = []string{"build", "-o", "/dev/null"}

	debugLog *log.Logger
)

func (*echoTransform) transform(input io.Reader, output io.Writer) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fmt.Fprintln(output, scanner.Text())
	}
}

func (*warningTransform) transform(input io.Reader, output io.Writer) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(output, lineNumRegex.ReplaceAllString(line, "${0}warning:"))
	}
}

func (xform *diffTransform) transform(input io.Reader, output io.Writer) {
	filename := ""
	currentLine := 0
	numDeleted := 0

	printWarning := func(text string) {
		fmt.Fprintf(output, "%s:%d:warning:%s", filename, currentLine, text)
		fmt.Fprintln(output, "")
	}

	printWarnings := func() {
		for ; numDeleted > 0; numDeleted-- {
			printWarning(xform.name + ":removed line")
			currentLine++
		}
	}

	scanner := bufio.NewScanner(input)

	// diff line
	scanner.Scan()
	line := scanner.Text()
	if !strings.HasPrefix(line, "diff ") {
		return
	}
	filename = strings.Split(line[5:], " ")[0]

	// --- +++ lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		if matches := diffHunkRegex.FindStringSubmatch(line); len(matches) > 0 {
			printWarnings()
			currentLine, _ = strconv.Atoi(matches[1])
		} else if strings.HasPrefix(line, "-") {
			numDeleted++
		} else if strings.HasPrefix(line, "+") {
			if numDeleted > 0 {
				printWarning(xform.name + ":changed: " + line[1:])
				numDeleted--
				currentLine++
			} else {
				printWarning(xform.name + ":added: " + line[1:])
			}
		} else {
			printWarnings()
			currentLine++
		}
	}
	printWarnings()
}

// buildCommand returns an *exec.Cmd which will build the file or
// package or, in the case of test files, the test binary. If the file
// appears to be part of a package, the entire package (or the test
// binary) is built otherwise only the single file is built. If the
// file is part of a package and has the flymake prefix, the file
// without the prefix is excluded from the build run.
func buildCommand(file string) flymakeCommand {
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

	return flymakeCommand{
		exec.Command("go", goArguments...),
		&echoTransform{},
		&echoTransform{},
	}
}

func vetCommand(file string) flymakeCommand {
	return flymakeCommand{
		exec.Command("go", "vet", file),
		&echoTransform{},
		&warningTransform{},
	}
}

func fmtCommand(file string) flymakeCommand {
	return flymakeCommand{
		exec.Command("gofmt", "-d", file),
		&diffTransform{"fmt"},
		&echoTransform{},
	}
}

func fixCommand(file string) flymakeCommand {
	return flymakeCommand{
		exec.Command("go", "tool", "fix", "-diff", file),
		&diffTransform{"fix"},
		&echoTransform{},
	}
}

// printCmdOutput start the given command and prints each line the
// command produces on standard output and standard error.
func (cmd flymakeCommand) printOutput() {
	stdout, err := cmd.cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = cmd.cmd.Start()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	runTransform := func(input io.Reader, xform transformer) {
		wg.Add(1)
		go func() { defer wg.Done(); xform.transform(input, os.Stdout) }()
	}

	runTransform(stdout, cmd.stdoutXform)
	runTransform(stderr, cmd.stderrXform)

	wg.Wait()
}

func init() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("%s some_file.go", path.Base(os.Args[0]))
	}

	testFile = flag.Args()[0]

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

func main() {
	var wg sync.WaitGroup

	runCmd := func(cmd flymakeCommand) {
		wg.Add(1)
		go func() { defer wg.Done(); cmd.printOutput() }()
	}

	runCmd(buildCommand(testFile))
	runCmd(vetCommand(testFile))
	runCmd(fmtCommand(testFile))
	runCmd(fixCommand(testFile))

	wg.Wait()
}
