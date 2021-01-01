package cmd

import (
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:              "k8ConsoleViewer",
	Short:            "An app for monitoring multiple namespaces.",
	PersistentPreRun: readConfig,
	Run:              runRootCmd,
}

var (
	buildVersion = ""
	buildTime    = ""
	namespace    string
	context      string
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags()
	rootCmd.Flags().StringVarP(&context, "context", "c", "", "context value, defaults to current context in .kube config")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace value")
	rootCmd.MarkFlagRequired("namespace")

	rootCmd.Version = fmt.Sprintf("%s (%s)", buildVersion, buildTime)
}

func runRootCmd(cmd *cobra.Command, args []string) {
	settings := viper.AllSettings()
	k8App, err := app.NewApp(context, namespace, settings)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	k8App.Run()
}

func readConfig(cmd *cobra.Command, args []string) {
	appDir, err := getAppDir()
	if err != nil {
		log.Fatal(err)
	}
	configFilePath := appDir + "/config.yaml"

	viper.SetConfigFile(configFilePath)
	err = viper.ReadInConfig()
	if err != nil {
		_, ok := err.(*os.PathError)
		if ok {
			fmt.Println("Config file not found, using defaults.")
		} else {
			log.Fatal(err)
		}
	}
}
