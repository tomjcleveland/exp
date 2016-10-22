// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/exp/ebnf"
)

// MaxInt values for finding minimum
const (
	MaxUint = ^uint(0)
	MaxInt  = int(MaxUint >> 1)
)

var fset = token.NewFileSet()
var start = flag.String("start", "Start", "name of start production")
var outfile = flag.String("out", "", "name out output EBNF file")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go tool ebnflint [flags] [filename]\n")
	flag.PrintDefaults()
	os.Exit(1)
}

// Markers around EBNF sections in .html files
var (
	open  = []byte(`<pre class="ebnf">`)
	close = []byte(`</pre>`)
)

func report(err error) {
	scanner.PrintError(os.Stderr, err)
	os.Exit(1)
}

func extractEBNF(src []byte) []byte {
	var buf bytes.Buffer

	for {
		// i = beginning of EBNF text
		i := bytes.Index(src, open)
		if i < 0 {
			break // no EBNF found - we are done
		}
		i += len(open)

		// write as many newlines as found in the excluded text
		// to maintain correct line numbers in error messages
		for _, ch := range src[0:i] {
			if ch == '\n' {
				buf.WriteByte('\n')
			}
		}

		// j = end of EBNF text (or end of source)
		j := bytes.Index(src[i:], close) // close marker
		if j < 0 {
			j = len(src) - i
		}
		j += i

		// copy EBNF text
		for k := i; k <= j; k++ {
			// If we encounter an opening html tag,
			// consume the tag before writing to buffer
			if src[k] == '<' {
				for src[k] != '>' {
					k++
				}
			} else {
				buf.Write([]byte{src[k]})
			}
		}

		// advance
		src = src[j:]
	}

	return buf.Bytes()
}

func main() {
	flag.Parse()

	var (
		name string
		r    io.Reader
	)
	switch flag.NArg() {
	case 0:
		name, r = "<stdin>", os.Stdin
	case 1:
		name = flag.Arg(0)
	default:
		usage()
	}

	if err := verify(name, *start, r, *outfile); err != nil {
		report(err)
	}
}

func verify(name, start string, r io.Reader, outFile string) error {
	if r == nil {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	src, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if filepath.Ext(name) == ".html" || bytes.Index(src, open) >= 0 {
		src = extractEBNF(src)
	}

	// Write EBNF to file
	if outFile != "" {
		err = writeEBNF(src, outFile)
		if err != nil {
			return err
		}
		fmt.Printf("generated EBNF file %q\n", outFile)
	}

	grammar, err := ebnf.Parse(name, bytes.NewBuffer(src))
	if err != nil {
		return err
	}

	if start == "Start" {
		// Find the start Production by trying everything
		prodErrMap := make(map[string]int)
		for prod := range grammar {
			count, _ := ebnf.Verify(grammar, prod)
			prodErrMap[prod] = count
		}
		min := MaxInt
		var startProd string
		for prod, count := range prodErrMap {
			if count < min {
				min = count
				startProd = prod
			}
		}
		if startProd == "" {
			return errors.New("failed to find start production")
		}
		start = startProd
	}

	fmt.Printf("using start production %q\n", start)
	_, err = ebnf.Verify(grammar, start)
	return err
}

func writeEBNF(src []byte, path string) error {
	for {
		new := bytes.Replace(src, []byte("\n\n"), []byte("\n"), -1)
		if len(new) < len(src) {
			src = new
		} else {
			break
		}
	}
	return ioutil.WriteFile(path, src, 0777)
}
