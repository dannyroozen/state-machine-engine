package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"state-machine-engine/adapters/servers"
)

func newServeCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run as a long-lived service accepting requests on the socket",
		RunE: func(cmd *cobra.Command, args []string) error {
			machinePath := viper.GetString("machine")
			runtimePath := viper.GetString("runtime")
			socketPath := viper.GetString("socket")

			service := rt.BuildService(machinePath, runtimePath)
			logger.Debug("starting server")
			if err := servers.StartAndListen(socketPath, service); err != nil {
				return fmt.Errorf("server failed: %w", err)
			}
			return nil
		},
	}
}
