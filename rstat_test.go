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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/maxim2266/strit"
)

func TestSSHCmdBuilder(t *testing.T) {
	type test struct {
		cmd []string
		exp string
	}

	tests := []test{
		{SSHCommand("192.168.0.16", "pi", "", 0), "ssh pi@192.168.0.16"},
		{SSHCommand("192.168.0.16", "pi", "raspberry", 0), "sshpass -p raspberry ssh pi@192.168.0.16"},
		{SSHCommand("192.168.0.16", "pi", "", 5), "ssh -o ConnectTimeout=5 pi@192.168.0.16"},
		{SSHCommand("192.168.0.16", "pi", "raspberry", 5), "sshpass -p raspberry ssh -o ConnectTimeout=5 pi@192.168.0.16"},
	}

	for _, tst := range tests {
		if s := strings.Join(tst.cmd, " "); s != tst.exp {
			t.Errorf("Invalid command string:\nexp: %q\ngot: %q", tst.exp, s)
			return
		}
	}
}

func TestNumberOfRecords(t *testing.T) {
	n, err := lc("valid-data")

	if err != nil {
		t.Error(err)
		return
	}

	root, err := pstree(cat("valid-data"))

	if err != nil {
		t.Error(err)
		return
	}

	var count int

	root.ForEach(func(pid int, _ map[string]string) {
		count++
	})

	if count != n-1 {
		t.Errorf("Unexpected number of records: %d instead of %d", count, n-1)
		return
	}
}

func TestTree(t *testing.T) {
	root, err := pstree(cat("valid-data"))

	if err != nil {
		t.Error(t)
		return
	}

	tree := make(map[int][]int, 10)

	root.ForEach(func(pid int, stats map[string]string) {
		tree[pid] = nil
		ppid, _ := strconv.Atoi(stats["PPID"])

		if ppid != 0 {
			tree[ppid] = append(tree[ppid], pid)
		}
	})

	for pid, list := range tree {
		if len(list) == 0 {
			delete(tree, pid)
		} else {
			sort.Ints(list)
		}
	}

	exp := map[int][]int{
		1:    {117, 120, 346, 347, 349, 351, 360, 365, 369, 372, 399, 439, 473, 486, 488, 498, 2239},
		346:  {350},
		399:  {2233},
		2233: {2245},
		2239: {2242},
		2245: {2247},
	}

	if len(tree) != len(exp) {
		t.Errorf("Invalid number of tree nodes: %d instead of %d", len(tree), len(exp))
		return
	}

	for pid, children := range exp {
		got, ok := tree[pid]

		if !ok {
			t.Errorf("PID %d not found", pid)
			return
		}

		if len(children) != len(got) {
			t.Errorf("Invalid number of children of pid %d: %d instead of %d", pid, len(got), len(children))
			return
		}

		for i, id := range children {
			if id != got[i] {
				t.Errorf("Child PID mismatch under %d: %d instead of %d", pid, got[i], id)
				return
			}
		}
	}
}

func TestParserErrorDetection(t *testing.T) {
	type test struct {
		file, msg string
	}

	tests := []test{
		{"invalid-number-of-columns", "Invalid number of columns is not detected"},
		{"negative-pid", "Negative PID is not detected"},
		{"bad-pid", "Bad PID is not detected"},
		{"missing-pid-column", "Negative PID is not detected"},
		{"no-pid-1", "Missing PID 1 is not detected"},
		{"no-such-file", "Wrong command 1 is not detected"},
	}

	for _, tst := range tests {
		if _, err := pstree(cat(tst.file)); err == nil {
			t.Error(tst.msg)
			return
		} /*else {
			println(err.Error())
		}*/
	}
}

func TestPlatform(t *testing.T) {
	root, err := ProcTree(nil, "%cpu", "%mem", "command")

	if err != nil {
		t.Error(err)
		return
	}

	var mem, cpu float64

	if root.Find(func(pid int, stat map[string]string) bool {
		cmd := stat["CMD"]

		if len(cmd) == 0 {
			t.Errorf("Command string for pid %d is not found or empty", pid)
			return true
		}

		if cmd[0] == '[' && cmd[len(cmd)-1] == ']' {
			t.Errorf("Unexpected kernel process %s: %q", pid, cmd)
			return true
		}

		val, err := strconv.ParseFloat(stat["%CPU"], 64)

		if err != nil {
			t.Errorf("Invalid floating point value for \"%CPU\": %q", stat["%CPU"])
			return true
		}

		cpu += val
		val, err = strconv.ParseFloat(stat["%MEM"], 64)

		if err != nil {
			t.Errorf("Invalid floating point value for \"%MEM\": %q", stat["%MEM"])
			return true
		}

		mem += val
		return false
	}) != nil {
		return
	}

	fmt.Printf("cpu: %.1f%%, memory: %.1f%%\n", cpu, mem)
}

const dataDir = "test-data/"

func cat(file string) []string {
	return []string{"cat", dataDir + file}
}

func lc(file string) (count int, err error) {
	err = strit.FromFile(dataDir + file)(func(_ []byte) error {
		count++
		return nil
	})

	return
}
