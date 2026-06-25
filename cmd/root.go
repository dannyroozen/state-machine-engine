package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"state-machine-engine/internal/logging"
)

var logger = logging.NewLogger("cmd")

var cfgFile string

func Execute(rt Runtime) {
	rootCmd := newRootCmd(rt)
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}

func newRootCmd(rt Runtime) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   executableName(),
		Short: "State machine engine CLI",
		Long:  "Run the state machine server, send requests, or generate config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("sending request")
			socketPath := viper.GetString("socket")
			input := viper.GetString("input")
			return rt.RunRequest(socketPath, input)
		},
	}

	cobra.OnInitialize(initConfig)

	// Remember to use two dashes for long flags, like: ./state-machine-engine --input '{"session_id":"", "input": {}}'
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "optional config file path (yaml/json/toml)")
	rootCmd.PersistentFlags().String("machine", "config/machine.json", "path to state machine definition json")
	rootCmd.PersistentFlags().String("runtime", "config/runtime.json", "path to runtime config json")
	rootCmd.PersistentFlags().String("socket", defaultSocketPath(), "path to the unix domain socket")
	rootCmd.PersistentFlags().String("input", "", `json input containing the request envelope {"session id":"", "input": {}}`)

	mustBindFlag(rootCmd, "config")
	mustBindFlag(rootCmd, "machine")
	mustBindFlag(rootCmd, "runtime")
	mustBindFlag(rootCmd, "socket")
	mustBindFlag(rootCmd, "input")

	rootCmd.AddCommand(newServeCmd(rt))
	rootCmd.AddCommand(newConfigCmd(rt))
	return rootCmd
}

func defaultSocketPath() string {
	return filepath.Join(os.TempDir(), "state-machine-engine.sock")
}

func executableName() string {
	name := filepath.Base(os.Args[0])
	return strings.TrimSuffix(name, filepath.Ext(name)) // handles ".exe" on Windows
}

func initConfig() {
	fmt.Println("initializing config")
	viper.SetEnvPrefix("SME")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			logger.Warn(fmt.Sprintf("failed to read config file %q: %v", cfgFile, err))
		} else {
			fmt.Println("successfully read config file")
		}
	}
}

func mustBindFlag(rootCmd *cobra.Command, name string) {
	if err := viper.BindPFlag(name, rootCmd.PersistentFlags().Lookup(name)); err != nil {
		logger.Fatal(fmt.Sprintf("failed to bind flag %q: %v", name, err))
	}
}
