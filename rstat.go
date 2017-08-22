/*
Copyright (c) 2017, Maxim Konakov
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY
OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE,
EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package rstat

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/maxim2266/strit"
)

// SSHCommand is a simple ssh command builder. Parameters 'host' and 'user' are mandatory,
// others are optional. The 'passw' parameter expects a password, in which case the composed command
// starts with 'sshpass', and if set to an empty string the result is an 'ssh' command.
// The last parameter specifies a timeout in seconds, set it to 0 for ssh default. It is generally
// recommended to set a timeout because the default settings for the 'ssh' command may have this parameter
// set to some high value. In practice a value of 5 seconds is usually sufficient for dealing with
// devices on local network. The function does not validate its input, instead relying on the 'ssh' program
// to produce an error if something is wrong.
func SSHCommand(host, user, passw string, seconds uint) (cmd []string) {
	if len(passw) > 0 {
		cmd = []string{"sshpass", "-p", passw, "ssh"}
	} else {
		cmd = []string{"ssh"}
	}

	if seconds > 0 {
		cmd = append(cmd, "-o", "ConnectTimeout="+strconv.FormatUint(uint64(seconds), 10))
	}

	cmd = append(cmd, user+"@"+host)
	return
}

// ProcNode is the node of the process tree. It contains the process pid, a map of metrics
// as produced by 'ps' program, and a list of child nodes.
type ProcNode struct {
	Pid      int
	Stats    map[string]string
	Children []*ProcNode
}

// ForEach applies the given function to each node of the process tree recursively.
func (root *ProcNode) ForEach(fn func(int, map[string]string)) {
	root.Find(func(pid int, stats map[string]string) bool {
		fn(pid, stats)
		return false
	})
}

// Find recursively applies the given predicate to each node of the process tree. It returns
// the first node for which the predicate is 'true'.
func (root *ProcNode) Find(pred func(int, map[string]string) bool) *ProcNode {
	if pred(root.Pid, root.Stats) {
		return root
	}

	// depth-first, assuming no loops
	node, stack := iterNodes([][]*ProcNode{root.Children}, pred)

	for len(stack) > 0 {
		node, stack = iterNodes(stack, pred)
	}

	return node
}

func iterNodes(stack [][]*ProcNode, pred func(int, map[string]string) bool) (*ProcNode, [][]*ProcNode) {
	nodes := stack[len(stack)-1]

	for i, node := range nodes {
		if pred(node.Pid, node.Stats) {
			return node, nil
		}

		if len(node.Children) > 0 {
			stack[len(stack)-1] = nodes[i+1:]
			return nil, append(stack, node.Children)
		}
	}

	return nil, stack[:len(stack)-1]
}

// ProcTree takes an ssh command and an optional list of columns, and returns
// a process tree starting at pid 1, or an error. The ssh command parameter is usually supplied
// by the SSHCommand function. It can be set to nil, in which case the 'ps' command gets invoked
// on the local machine. The list of columns should include only those expected by the 'ps' command
// on the target machine, try 'ps L' for the full list. An empty column list results in 'ps -eF'
// invocation. All the columns are returned 'as-is', whatever the 'ps' command on the target machine
// may produce. For convenience and in order to be able to build a process tree, the values of 'PID'
// and 'PPID' are always included.
func ProcTree(ssh []string, columns ...string) (*ProcNode, error) {
	return pstree(concat(ssh, makePsCommand(columns)))
}

func pstree(cmd []string) (*ProcNode, error) {
	//println(strings.Join(cmd, " "))

	var parser psParser

	if err := nonEmptyLines(strit.FromCommand(cmd[0], cmd[1:]...)).Parse(&parser); err != nil {
		return nil, err
	}

	return buildProcTree(parser.stats)
}

// 'ps' command builder
func makePsCommand(columns []string) []string {
	if len(columns) == 0 {
		return []string{"ps", "-ewwF"}
	}

	// process the column list to remove duplicates
	m := make(map[string]int, len(columns))
	var cmd bool

	for _, c := range columns {
		switch c {
		// 'cmd' column must be at the end of the list to avoid truncation
		case "args", "cmd", "command":
			cmd = true
		// 'pid' and 'ppid' are always the first two columns
		case "pid", "ppid":
			// skip
		default:
			m[c] = 0
		}
	}

	// build column list
	buff := bytes.NewBufferString("pid,ppid")

	for c := range m {
		buff.WriteByte(',')
		buff.WriteString(c)
	}

	if cmd {
		buff.WriteString(",cmd")
	}

	// done
	return []string{"ps", "-ewwo", buff.String()}
}

// parser for 'ps' output
type psParser struct {
	header []string
	stats  []map[string]string
}

// parser entry point, reads table header
func (p *psParser) Enter(line []byte) (strit.ParserFunc, error) {
	p.stats = make([]map[string]string, 0, 100)

	if p.header = strings.Fields(string(line)); len(p.header) < 2 {
		return nil, fmt.Errorf("Invalid header in 'ps' output: %q", strings.Join(p.header, " "))
	}

	//println(strings.Join(p.header, " "))
	return p.read, nil
}

// reads the 'ps' data after the header
func (p *psParser) read(line []byte) (strit.ParserFunc, error) {
	fields := wsRe.Split(string(line), len(p.header))

	if len(fields) != len(p.header) {
		return nil, fmt.Errorf("Invalid number of columns (%d instead of %d): %q",
			len(fields), len(p.header), strings.Join(fields, " "))
	}

	m := make(map[string]string, len(p.header))

	for i, s := range fields {
		m[p.header[i]] = s
	}

	p.stats = append(p.stats, m)
	return p.read, nil
}

var wsRe = regexp.MustCompile(`\s+`)

// parser finaliser
func (p *psParser) Done(err error) error {
	if err != nil {
		return mapCmdError(err)
	}

	return nil
}

// process tree builder
func buildProcTree(stats []map[string]string) (*ProcNode, error) {
	// build a map from 'pid' to *ProcNode
	nodes := make(map[int]*ProcNode, len(stats))

	for _, stat := range stats {
		pid, err := getPid(stat, "PID")

		if err != nil {
			return nil, err
		}

		nodes[pid] = &ProcNode{Pid: pid, Stats: stat}
	}

	// build process tree
	for _, node := range nodes {
		ppid, err := getPid(node.Stats, "PPID")

		if err != nil {
			return nil, err
		}

		if parent := nodes[ppid]; parent != nil {
			parent.Children = append(parent.Children, node)
		}
	}

	// return the root (pid 1); this ignores every process that is not a descendant of pid 1,
	// thus filtering out kernel threads
	// Q: Is there a way to filter out kernel threads using just 'ps' options?
	if root := nodes[1]; root != nil {
		return root, nil
	}

	// root not found
	return nil, errors.New("Root process with pid 1 is not found")
}

// reads pid or similar non-negative integer from string map
func getPid(stats map[string]string, key string) (val int, err error) {
	str := stats[key]

	if len(str) == 0 {
		err = fmt.Errorf("Metric %q is not found or empty", key)
	} else if val, err = strconv.Atoi(str); err != nil {
		if e, ok := err.(*strconv.NumError); ok {
			err = fmt.Errorf("Parsing metric %s %q: %s", key, str, e.Err)
		}
	} else if val < 0 {
		err = fmt.Errorf("Parsing metric %s %q: negative value", key, str)
	}

	return
}

// nonEmptyLines makes a new iterator combining white-space trimming and empty lines filtering
func nonEmptyLines(iter strit.Iter) strit.Iter {
	return func(fn strit.Func) error {
		return iter(func(line []byte) (err error) {
			if line = bytes.TrimSpace(line); len(line) > 0 {
				err = fn(line)
			}

			return
		})
	}
}

// error mapper for parser
func mapCmdError(err error) error {
	switch e := err.(type) {
	case *strit.ExitError:
		msg := e.Stderr

		// cut off 'usage' strings, if any
		if i := strings.IndexRune(msg, '\n'); i > 0 {
			msg = strings.TrimSpace(msg[:i])
		}

		// done
		return errors.New(cutErrPrefix(msg))

	default:
		return errors.New(cutErrPrefix(err.Error()))
	}
}

var matchErrPrefix = regexp.MustCompile(`^[[:alpha:]]+:\s+`).FindStringIndex

func cutErrPrefix(msg string) string {
	if loc := matchErrPrefix(msg); len(loc) > 0 {
		return msg[loc[1]:]
	}

	return msg
}

func concat(a, b []string) (res []string) {
	res = make([]string, len(a)+len(b))

	copy(res[copy(res, a):], b)
	return
}
