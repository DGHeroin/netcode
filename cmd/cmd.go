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
    // 命令拦截
    if len(os.Args) == 2 {
        filename := os.Args[1]
        if st, err := os.Stat(filename); err == nil {
            if !st.IsDir() {
                os.Args[1] = "run"
                os.Args = append(os.Args, filename)
            }
        }
    }
    rootCmd.AddCommand(run.Cmd)
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(-1)
    }
}
