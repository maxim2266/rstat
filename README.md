# rstat

[![GoDoc](https://godoc.org/github.com/maxim2266/rstat?status.svg)](https://godoc.org/github.com/maxim2266/rstat)
[![Go report](http://goreportcard.com/badge/maxim2266/rstat)](http://goreportcard.com/report/maxim2266/rstat)

Library `rstat` provides basic functionality for periodical health check of IoT devices running Linux.
It has an API for invoking `ps` command on a remote device via `ssh`, returning a process tree with the
requested metrics of each process.

Usage example:

```Go
ssh := rstat.SSHCommand("192.168.0.16", "pi", "raspberry", 5)
root, err := rstat.ProcTree(ssh, "%cpu", "%mem", "cmd")

if err != nil {
	return err
}

root.ForEach(func(pid int, stats map[string]string) {
	// for this example just print the metrics
	fmt.Printf("%d %s %s %s %q\n", pid, stats["PPID"], stats["%CPU"], stats["%MEM"], stats["CMD"])
})

```

### Project status
The project is in a alpha state. Tested on Linux Mint 18.2. Go version 1.8.

##### Platform: Linux
##### License: BSD
