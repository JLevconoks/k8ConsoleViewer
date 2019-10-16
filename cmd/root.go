package cmd

import (
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/app"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "k8ConsoleViewer",
	Short: "An app for monitoring multiple namespaces.",
	Run:   runRootCmd,
}

var (
	buildVersion string = ""
	buildTime    string = ""
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
	rootCmd.Flags().StringVarP(&context, "context", "c", "", "context value")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace value")
	rootCmd.MarkFlagRequired("context")
	rootCmd.MarkFlagRequired("namespace")

	rootCmd.Version = fmt.Sprintf("%s (%s)", buildVersion, buildTime)
}

func runRootCmd(cmd *cobra.Command, args []string) {
	k8App, err := app.NewApp(context, namespace)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	k8App.Run()
}
