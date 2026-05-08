package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version se sobrescribe con `-ldflags "-X main.version=<tag>"` en el build.
var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:           "vexd",
		Short:         "Vex execution engine",
		Long:          "vexd ejecuta una pipeline definida por un RequestInput JSON y termina con exit code 0/1.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddCommand(newRunCommand())
	rootCmd.AddCommand(newVersionCommand(version))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
