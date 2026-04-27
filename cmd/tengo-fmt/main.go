// tengo-fmt formats Tengo source files.
//
// Usage:
//
//	tengo-fmt [flags] [file ...]
//
// With no files, reads from stdin and writes to stdout.
// With files, rewrites each file in place (use -w=false to write to stdout).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/tengolang/tengo/v3/format"
)

func main() {
	write := flag.Bool("w", true, "write result to source file instead of stdout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tengo-fmt [-w] [file ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		if err := processReader(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	exitCode := 0
	for _, path := range flag.Args() {
		if err := processFile(path, *write); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func processReader(r io.Reader, w io.Writer) error {
	src, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	out, err := format.Format(src)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func processFile(path string, write bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := format.Format(src)
	if err != nil {
		return err
	}
	if write {
		return os.WriteFile(path, out, 0644)
	}
	_, err = os.Stdout.Write(out)
	return err
}
