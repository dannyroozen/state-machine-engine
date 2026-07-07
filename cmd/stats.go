package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newStatsCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Get session statistics from the running service",
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath := viper.GetString("socket")
			return rt.RunRequest(socketPath, "", "/stats")
		},
	}
}
