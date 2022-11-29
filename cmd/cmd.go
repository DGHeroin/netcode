package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "netcode/cmd/run"
    "os"
)

var (
    rootCmd = &cobra.Command{
        Use: "netcode",
    }
)

func Run() {
    rootCmd.AddCommand(run.Cmd)
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(-1)
    }
}
