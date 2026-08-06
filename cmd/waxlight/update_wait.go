package main

import (
	"os"
	"strconv"
)

// updateWaitPID scans the command-line arguments for the
// "--update-wait-pid <pid>" pair that installer_linux.go passes to the
// restarted portable binary. It returns 0 when the flag is absent, malformed,
// or the value is not a positive process ID.
func updateWaitPID() int {
	return updateWaitPIDFromArgs(os.Args[1:])
}

func updateWaitPIDFromArgs(args []string) int {
	for index := 0; index < len(args)-1; index++ {
		if args[index] != "--update-wait-pid" {
			continue
		}
		value, err := strconv.Atoi(args[index+1])
		if err != nil || value <= 0 {
			return 0
		}
		return value
	}
	return 0
}
