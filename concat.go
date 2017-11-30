// concat is a first-pass trial implementation of part of a Forth-like language interpreter.
// The main purpose of the package was to practice Go basics.
// It was inspired by Rob Pike's talk on lexical scanning and Thorsten Ball's book - "Writing an Interpreter in Go".
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

type token struct {
	typ tokenType
	val string
}

type tokenType int

func (t token) String() string {
	return fmt.Sprintf("%v: %v", t.typ, t.val)
}

const (
	tokenUnknown  tokenType = iota // don't recognise the input
	tokenInt                       // integer
	tokenPlus                      // +
	tokenMinus                     // -
	tokenMultiply                  // *
	tokenDivide                    // /
	tokenDot                       // dot - pop and print
	tokenPS                        // .S print the stack
	tokenEOL                       // end of line
)

type lexer struct {
	line  string
	start int
	pos   int
}

func (lxr *lexer) init(text string) {
	lxr.line = text
	lxr.start = 0
	lxr.pos = 0
}

func (lxr *lexer) next() token {
	lxr.skipWhiteSpace()
	if lxr.pos >= len(lxr.line) {
		return token{typ: tokenEOL}
	}
	ch := lxr.line[lxr.pos]
	sch := string(ch)
	switch {
	case ch == '+':
		lxr.pos++
		return token{typ: tokenPlus, val: sch}
	case ch == '-':
		lxr.pos++
		return token{typ: tokenMinus, val: sch}
	case ch == '*':
		lxr.pos++
		return token{typ: tokenMultiply, val: sch}
	case ch == '/':
		lxr.pos++
		return token{typ: tokenDivide, val: sch}
	case ch == '.':
		if lxr.peek() == 'S' {
			lxr.pos += 2
			return token{typ: tokenPS}
		} else {
			lxr.pos++
			return token{typ: tokenDot, val: sch}
		}
	case isDigit(ch):
		lxr.start = lxr.pos
		for {
			lxr.pos++
			if lxr.pos >= len(lxr.line) || !isDigit(lxr.line[lxr.pos]) {
				lxr.pos--
				break
			}
		}
		lxr.pos++
		return token{typ: tokenInt, val: lxr.line[lxr.start:lxr.pos]}
	default:
		lxr.start = lxr.pos
		for {
			lxr.pos++
			if lxr.pos >= len(lxr.line) || isWhiteSpace(lxr.line[lxr.pos]) {
				lxr.pos--
				break
			}
		}
		lxr.pos++
		return token{typ: tokenUnknown, val: lxr.line[lxr.start:lxr.pos]}
	}
}

func (lxr *lexer) skipWhiteSpace() {
	for {
		if lxr.pos < len(lxr.line) && lxr.line[lxr.pos] == ' ' {
			lxr.pos++
		} else {
			break
		}
	}
}

func (lxr *lexer) peek() byte {
	if lxr.pos < len(lxr.line)-1 {
		return lxr.line[lxr.pos+1]
	} else {
		return ' '
	}
}

func isDigit(n byte) bool {
	if n >= '0' && n <= '9' {
		return true
	} else {
		return false
	}
}

func isWhiteSpace(ch byte) bool {
	if ch == ' ' || ch == '\t' {
		return true
	} else {
		return false
	}
}

const stackSize = 1000

// This is an old-fashioned fixed-length stack which avoids the usual incessant memory alloc/dealloc.
// Note that I even waste the first (zeroth) entry so I can use top = 0 to indicate empty.
type stack struct {
	top   uint
	frame [stackSize + 1]struct {
		used    bool
		payload token
		prev    uint
	}
}

func (s *stack) push(t token) {
	for i := 1; i <= stackSize; i++ {
		if !s.frame[i].used {
			s.frame[i].used = true
			s.frame[i].payload = t
			s.frame[i].prev = s.top
			s.top = uint(i)
			return
		}
	}
	panic(fmt.Errorf("Stack overflow"))
}

func (s *stack) pop() token {
	if s.top == 0 {
		panic(fmt.Errorf("Stack underflow"))
	}
	ret := s.frame[s.top].payload
	s.frame[s.top].used = false
	s.top = s.frame[s.top].prev
	return ret
}

func (s *stack) prin() {
	fr := s.top
	for fr != 0 {
		fmt.Printf("%v %v %v\n", fr, s.frame[fr].payload.typ, s.frame[fr].payload.val)
		fr = s.frame[fr].prev
	}
}

func main() {
	ch := make(chan token)
	go getTokens(ch)

	// Set up a map to generalise the application of the dyadic operators.
	fMap := make(map[tokenType]func(int, int) int)
	fMap[tokenPlus] = func(x, y int) int { return x + y }
	fMap[tokenMinus] = func(x, y int) int { return x - y }
	fMap[tokenMultiply] = func(x, y int) int { return x * y }
	fMap[tokenDivide] = func(x, y int) int { return x / y }

	// Our subset of Forth recognises unsigned integers, +, -, *, /, ., and .S
	var stak stack
	for tok := range ch {
		switch tok.typ {
		case tokenDot:
			fmt.Printf("%v\n", stak.pop())
		case tokenPS:
			stak.prin()
		case tokenInt:
			stak.push(tok)
		case tokenUnknown:
			fmt.Printf("Unrecognised input %v has been ignored\n", tok.val)
		default:
			t1 := stak.pop()
			v1, err := strconv.Atoi(t1.val)
			if err != nil {
				panic(fmt.Errorf("Token maybe not integer: %v\n", t1))
			}
			t2 := stak.pop()
			v2, err := strconv.Atoi(t2.val)
			if err != nil {
				panic(fmt.Errorf("Token maybe not integer: %v\n", t2))
			}
			rs := strconv.Itoa(fMap[tok.typ](v1, v2))
			stak.push(token{typ: tokenInt, val: rs})
		}
	}
}

func getTokens(c chan<- token) {
	var lxr lexer
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			panic(fmt.Errorf("reading standard input:%v", err))
		}
		lxr.init(scanner.Text())
		for {
			t := lxr.next()
			if t.typ == tokenEOL {
				break
			}
			c <- t
		}
	}
	close(c)
}
