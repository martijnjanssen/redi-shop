package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/martijnjanssen/redi-shop/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile string
	service string
	backend string
	port    string

	rootCmd = &cobra.Command{
		Use:   "redi",
		Short: "redi is a microservice shop implementation",
		Long: `A fast and resilient microservice implementation with different
                backends.`,
		Run: func(cmd *cobra.Command, args []string) {
			server.Start()
		},
	}
)

// Initialize commands
func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./redi.yaml)")
	rootCmd.Flags().StringVarP(&service, "service", "s", "", "Service to start (user, stock, order, payment)")
	rootCmd.Flags().StringVarP(&backend, "backend", "b", "", "Backend to use (postgres, redis)")
	rootCmd.Flags().StringVarP(&port, "port", "p", "", "Port to listen in")
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath("./")
		viper.SetConfigName("redi")
	}

	// Bind service flag to env values
	err := viper.BindPFlag("service", rootCmd.Flags().Lookup("service"))
	if err != nil {
		logrus.WithError(err).Fatal("unable to bind service flag to config value")
	}
	err = viper.BindPFlag("backend", rootCmd.Flags().Lookup("backend"))
	if err != nil {
		logrus.WithError(err).Fatal("unable to bind backend flag to config value")
	}
	err = viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	if err != nil {
		logrus.WithError(err).Fatal("unable to bind port flag to config value")
	}

	// Default config values
	viper.SetDefault("port", "8000")

	viper.SetDefault("postgres.url", "localhost")
	viper.SetDefault("postgres.port", "5432")
	viper.SetDefault("postgres.username", "postgres")
	viper.SetDefault("postgres.password", "postgres")
	viper.SetDefault("postgres.database", "redi")

	viper.SetDefault("redis.url", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "redis")

	viper.SetDefault("broker.url", "localhost")
	viper.SetDefault("broker.port", "6379")
	viper.SetDefault("broker.password", "redis")

	viper.SetDefault("url.user", "localhost")
	viper.SetDefault("url.order", "localhost")
	viper.SetDefault("url.stock", "localhost")
	viper.SetDefault("url.payment", "localhost")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		logrus.WithField("file", viper.ConfigFileUsed()).Info("Loaded config")
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
