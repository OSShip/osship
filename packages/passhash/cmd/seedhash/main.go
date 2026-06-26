package main

import (
	"fmt"
	"os"

	"github.com/osship/osship/packages/passhash"
)

func main() {
	password := "password123"
	if len(os.Args) > 1 {
		password = os.Args[1]
	}
	salt, hash, err := passhash.HashPasswordPair(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash password: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\t%s\n", salt, hash)
}
