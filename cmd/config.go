package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newConfigCmd(rt Runtime) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Launch config assistant workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			machinePath := viper.GetString("machine")
			runtimePath := viper.GetString("runtime")
			model := viper.GetString("model")
			return rt.RunConfigAssistant(machinePath, runtimePath, model)
		},
	}

	configCmd.Flags().String("model", "", "llm model name")
	configCmd.Flags().Bool("non-interactive", false, "run config assistant without prompts")

	if err := viper.BindPFlag("model", configCmd.Flags().Lookup("model")); err != nil {
		logger.Fatal(err.Error())
	}
	if err := viper.BindPFlag("non-interactive", configCmd.Flags().Lookup("non-interactive")); err != nil {
		logger.Fatal(err.Error())
	}

	return configCmd
}
