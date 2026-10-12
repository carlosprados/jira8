package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Confirm asks before a destructive action. It returns false when the user
// declines. Without a terminal on stdin it fails instead of reading EOF as
// "no": a script or agent would otherwise get exit 0 for a deletion that never
// happened.
func Confirm(prompt string, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("refusing to delete without --yes: stdin is not a terminal")
	}
	fmt.Printf("%s [y/N] ", prompt)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return false, nil
	}
	return true, nil
}
