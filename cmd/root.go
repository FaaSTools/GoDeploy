package cmd

import (
	"godeploy/shared"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "godeploy",
	Short: "CLI tool to deploy serverless functions",
	Long: `This tool can be used as a standalone application to deploy 
zipped serverless functions to multiple FaaS providers (AWS, GCP).

	For example:
	
	godeploy deploy -f "deployment.yaml"
`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.GoDeploy.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Find working directory.
	wd, err := os.Getwd()
	cobra.CheckErr(err)
	//Set type for all configuration files to .yaml
	viper.AddConfigPath(wd)
	viper.SetConfigType(shared.DefaultFileExtension)

	//viper.AutomaticEnv() read in environment variables that match
}
