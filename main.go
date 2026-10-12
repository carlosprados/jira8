package main

import (
	"os"

	"github.com/amplia/jira8/cmd"
)

func main() {
	os.Exit(cmd.ExitCode(cmd.Execute()))
}
