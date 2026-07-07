package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDeleteExpiredCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-expired",
		Short: "Delete expired sessions from the running service",
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath := viper.GetString("socket")
			return rt.RunRequest(socketPath, "", "/sessions/expired")
		},
	}
}
