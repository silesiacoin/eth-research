package main

import (
	"bufio"
	"fmt"
	"strings"
)

var scanTests = []string{
	"",
	"a",
	"¼",
	"☹",
	"\x81",   // UTF-8 error
	"\uFFFD", // correctly encoded RuneError
	"abcdefgh",
	"abc def\n\t\tgh    ",
	"abc¼☹\x81\uFFFD日本語\x82abc",
}

func ScanByte() {
	for n, test := range scanTests {
		buf := strings.NewReader(test)
		s := bufio.NewScanner(buf)
		s.Split(bufio.ScanBytes)
		var i int
		for i = 0; s.Scan(); i++ {
			if b := s.Bytes(); len(b) != 1 || b[0] != test[i] {
				fmt.Printf("#%d: %d: expected %q got %q", n, i, test, b)
			}
		}
		if i != len(test) {
			fmt.Printf("#%d: termination expected at %d; got %d", n, len(test), i)
		}
		err := s.Err()
		if err != nil {
			fmt.Printf("#%d: %v", n, err)
		}
	}
}

func main() {
	ScanByte()
}
