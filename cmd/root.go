/*
	Copyright 2023 Markus Papenbrock
*/

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	checkCmd "github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/check"
	migrateCmd "github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/migrate"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/repair"
	grpcServer "github.com/mpapenbr/iracelog-service-manager-go/pkg/cmd/server/grpc"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/version"
)

const envPrefix = "ISM"

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "ism",
	Short:   "Message processing backend for iRacelog",
	Long:    ``,
	Version: version.FullVersion,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// util.SetupLogger(config.DefaultCliArgs())
		// if _, err := log.InitLoggerManager(config.DefaultCliArgs()); err != nil {
		// 	fmt.Fprintf(os.Stderr, "Error initializing logger: %v", err)
		// 	os.Exit(1)
		// }
		logConfig := log.DefaultDevConfig()
		if config.LogConfig != "" {
			var err error
			logConfig, err = log.LoadConfig(config.LogConfig)
			if err != nil {
				log.Fatal("could not load log config", log.ErrorField(err))
			}
		}
		l := log.NewWithConfig(logConfig, config.LogLevel)
		cmd.SetContext(log.AddToContext(context.Background(), l))
		log.ResetDefault(l)
	},

	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.ism.yml)")

	rootCmd.PersistentFlags().StringVar(&config.DB, "db",
		"postgresql://DB_USERNAME:DB_USER_PASSWORD@DB_HOST:5432/iracelog",
		"Connection string for the database")

	rootCmd.PersistentFlags().StringVar(&config.WaitForServices,
		"wait-for-services",
		"15s",
		"Duration to wait for other services to be ready")
	rootCmd.PersistentFlags().StringVar(&config.LogConfig,
		"log-config",
		"",
		"configuration file for logger")
	rootCmd.PersistentFlags().StringVar(&config.LogLevel,
		"log-level",
		"info",
		"controls the log level (debug, info, warn, error, fatal)")

	// add commands here
	rootCmd.AddCommand(migrateCmd.NewMigrateCmd())
	rootCmd.AddCommand(grpcServer.NewServerCmd())
	rootCmd.AddCommand(checkCmd.NewCheckCmd())
	rootCmd.AddCommand(repair.NewRepairCmd())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".ism" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ism")
	}

	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// we want all commands to be processed by the bindFlags function
	// even those N levels deep
	cmds := []*cobra.Command{}
	collectCommands(rootCmd, &cmds)

	for _, cmd := range cmds {
		bindFlags(cmd, viper.GetViper())
	}
}

func collectCommands(cmd *cobra.Command, commands *[]*cobra.Command) {
	*commands = append(*commands, cmd)
	for _, subCmd := range cmd.Commands() {
		collectCommands(subCmd, commands)
	}
}

// Bind each cobra flag to its associated viper configuration
// (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their
		// equivalent keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			if err := v.BindEnv(f.Name,
				fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)); err != nil {
				fmt.Fprintf(os.Stderr, "Could not bind env var %s: %v", f.Name, err)
			}
		}
		// Apply the viper config value to the flag when the flag is not set and viper
		// has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
				fmt.Fprintf(os.Stderr, "Could set flag value for %s: %v", f.Name, err)
			}
		}
	})
}
