package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CodeToole/waitaminutedigital_go/internal/auth"
)

func main() {
	fmt.Fprint(os.Stderr, "Admin password: ")
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(password) == 0 {
		fmt.Fprintln(os.Stderr, "Read password:", err)
		os.Exit(1)
	}
	password = strings.TrimRight(password, "\r\n")
	if password == "" {
		fmt.Fprintln(os.Stderr, "Password cannot be empty.")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hash password:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
