package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// tcsetpgrpTo sets the terminal's foreground process group, the same thing
// the shell does when it takes back a stopped job.
func tcsetpgrpTo(fd int, pgrp int) error {
	return unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, pgrp)
}

// ttyForegroundPgrp reports the terminal's current foreground pgrp.
func ttyForegroundPgrp(fd int) (int, error) {
	p, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	return p, err
}

var _ = os.Stdin // keep os imported if unused elsewhere
