package main

import (
	"fmt"
	"os"

	"github.com/cedriclam/k8snssetup/cmd/k8snssetup/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}
