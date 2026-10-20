package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// A real native executable keeps argv and lifecycle tests independent of shell
// syntax. Each invocation is selected explicitly, never during ordinary tests.
func apiProcess(mode string, args ...string) []string {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return append([]string{executable, "-test.run=^TestAPIProcess$", "--", "webmux-helper", mode}, args...)
}
func TestAPIProcess(t *testing.T) {
	at := -1
	for i, arg := range os.Args {
		if arg == "webmux-helper" {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	args := os.Args[at+1:]
	switch args[0] {
	case "idle":
		time.Sleep(time.Minute)
	case "print":
		fmt.Print(strings.Join(args[1:], ""))
	case "wait":
		rawTestInput()
		fmt.Print(args[1])
		time.Sleep(time.Minute)
	case "argv":
		cwd, _ := os.Getwd()
		data, _ := json.Marshal(struct {
			Cwd  string
			Args []string
		}{cwd, args[1:]})
		if err := os.WriteFile("argv-result.json", data, 0600); err != nil {
			os.Exit(2)
		}
		fmt.Printf("cwd=%s\n", cwd)
		for _, arg := range args[1:] {
			fmt.Printf("<%s>\n", arg)
		}
	case "after":
		rawTestInput()
		fmt.Print("READY")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Print("after-" + strings.TrimRight(line, "\r\n"))
		time.Sleep(time.Minute)
	case "record", "gated-record":
		rawTestInput()
		fmt.Print("READY")
		if args[0] == "gated-record" {
			for {
				if _, err := os.Stat(args[1] + ".go"); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		f, err := os.Create(args[1])
		if err != nil {
			os.Exit(2)
		}
		_, _ = io.Copy(f, os.Stdin)
		_ = f.Close()
	}
	os.Exit(0)
}
