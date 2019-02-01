# rstat

[![GoDoc](https://godoc.org/github.com/maxim2266/rstat?status.svg)](https://godoc.org/github.com/maxim2266/rstat)
[![Go report](http://goreportcard.com/badge/maxim2266/rstat)](http://goreportcard.com/report/maxim2266/rstat)

Library `rstat` provides basic functionality for periodical health check of IoT devices running Linux.
It has an API for invoking `ps` command on a remote device via `ssh`, returning a process tree with the
requested metrics for each process.

#### Usage example:

```Go
// compose ssh command with the given ip, user, password and timeout in seconds
ssh := rstat.SSHCommand("192.168.0.16", "pi", "raspberry", 5)
// request process tree with 3 metrics per process
root, err := rstat.ProcTree(ssh, "%cpu", "%mem", "cmd")

if err != nil {
	return err
}

// iterate the process tree
root.ForEach(func(node *rstat.ProcNode) {
	// just for example, print the metrics
	fmt.Printf("%d %s %s %s %q\n", node.Pid, node.ParentPid, node.Stats["%CPU"], node.Stats["%MEM"], node.Stats["CMD"])
})

```

### Project status
The project is in a alpha state. Tested on Linux Mint 18.2. Go version 1.8.

##### Platform: Linux
##### License: BSD
