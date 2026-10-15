// Command gcc-wrapper forwards cgo compiler invocations to a selected GCC and
// strips a linker option unsupported by older MinGW binutils.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	compiler := os.Getenv("MINGW_GCC")
	if compiler == "" {
		compiler = "gcc"
	}
	args := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		if arg == "-Wl,--high-entropy-va" {
			continue
		}
		args = append(args, arg)
	}
	cmd := exec.Command(compiler, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
