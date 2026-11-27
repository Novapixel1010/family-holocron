// Package gedcom reads and writes the GEDCOM 5.5.1 line format as a generic
// record tree. Mapping to Holocron's model lives in server/module/exchange.
package gedcom

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Node is one GEDCOM line with its sub-lines.
type Node struct {
	Level    int
	Xref     string // "@I1@" on records
	Tag      string
	Value    string
	Children []*Node
}

// Child returns the first child with tag, or nil.
func (n *Node) Child(tag string) *Node {
	for _, c := range n.Children {
		if c.Tag == tag {
			return c
		}
	}
	return nil
}

// All returns every child with tag.
func (n *Node) All(tag string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// Val returns the value of the first child with tag ("" if missing).
func (n *Node) Val(tag string) string {
	if c := n.Child(tag); c != nil {
		return c.Text()
	}
	return ""
}

// Text returns the node value with CONT/CONC continuations applied.
func (n *Node) Text() string {
	var b strings.Builder
	b.WriteString(n.Value)
	for _, c := range n.Children {
		switch c.Tag {
		case "CONT":
			b.WriteString("\n" + c.Value)
		case "CONC":
			b.WriteString(c.Value)
		}
	}
	return b.String()
}

// Parse reads a GEDCOM stream and returns its level-0 records.
func Parse(r io.Reader) ([]*Node, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var roots []*Node
	var stack []*Node
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r\n")
		if lineNo == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		line = strings.TrimLeft(line, " \t")
		if line == "" {
			continue
		}
		n, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if n.Level == 0 {
			roots = append(roots, n)
			stack = []*Node{n}
			continue
		}
		if len(stack) == 0 {
			return nil, fmt.Errorf("line %d: data before first record", lineNo)
		}
		if n.Level > len(stack) {
			return nil, fmt.Errorf("line %d: level %d skips a level", lineNo, n.Level)
		}
		stack = stack[:n.Level]
		parent := stack[len(stack)-1]
		parent.Children = append(parent.Children, n)
		stack = append(stack, n)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no GEDCOM records found")
	}
	if roots[0].Tag != "HEAD" {
		return nil, fmt.Errorf("not a GEDCOM file (missing HEAD record)")
	}
	return roots, nil
}

func parseLine(line string) (*Node, error) {
	parts := strings.SplitN(line, " ", 2)
	lvl, err := strconv.Atoi(parts[0])
	if err != nil || lvl < 0 || lvl > 99 {
		return nil, fmt.Errorf("invalid level %q", parts[0])
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("missing tag")
	}
	rest := parts[1]
	n := &Node{Level: lvl}
	if strings.HasPrefix(rest, "@") {
		end := strings.Index(rest[1:], "@")
		if end < 0 {
			return nil, fmt.Errorf("unterminated xref")
		}
		n.Xref = rest[:end+2]
		rest = strings.TrimLeft(rest[end+2:], " ")
	}
	tv := strings.SplitN(rest, " ", 2)
	n.Tag = strings.ToUpper(tv[0])
	if len(tv) == 2 {
		n.Value = tv[1]
	}
	if n.Tag == "" {
		return nil, fmt.Errorf("missing tag")
	}
	return n, nil
}

// Writer emits GEDCOM lines.
type Writer struct {
	w   *bufio.Writer
	err error
}

// NewWriter wraps w.
func NewWriter(w io.Writer) *Writer { return &Writer{w: bufio.NewWriter(w)} }

// Line writes one line; multi-line values are split with CONT, long ones with CONC.
func (g *Writer) Line(level int, xref, tag, value string) {
	if g.err != nil {
		return
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lv, tg := level, tag
		xr := xref
		if i > 0 {
			lv, tg, xr = level+1, "CONT", ""
		}
		chunks := chunk(l, 200)
		for j, c := range chunks {
			if j > 0 {
				lv, tg, xr = level+1, "CONC", ""
			}
			g.write(lv, xr, tg, c)
		}
	}
}

func (g *Writer) write(level int, xref, tag, value string) {
	var b strings.Builder
	b.WriteString(strconv.Itoa(level))
	if xref != "" {
		b.WriteString(" " + xref)
	}
	b.WriteString(" " + tag)
	if value != "" {
		b.WriteString(" " + value)
	}
	b.WriteString("\n")
	_, g.err = g.w.WriteString(b.String())
}

// Flush flushes buffered output.
func (g *Writer) Flush() error {
	if g.err != nil {
		return g.err
	}
	return g.w.Flush()
}

func chunk(s string, n int) []string {
	r := []rune(s)
	if len(r) <= n {
		return []string{s}
	}
	var out []string
	for len(r) > n {
		cut := n
		// Avoid splitting right before a space (CONC drops leading spaces in some readers).
		for cut > 1 && r[cut] == ' ' {
			cut--
		}
		out = append(out, string(r[:cut]))
		r = r[cut:]
	}
	return append(out, string(r))
}
