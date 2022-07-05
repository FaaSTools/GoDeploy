package cmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	my_aws "godeploy/aws"
	"godeploy/google"
	"godeploy/shared"
	google2 "golang.org/x/oauth2/google"
	"os"
	"strings"
	"sync"
)

var deploymentFile string
var deploymentDtos []shared.DeploymentDto
var credentials shared.CredentialsHolder

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys your functions to different FaaS providers",
	Long: `Acts as the general command that lets you upload serverless functions to multiple FaaS providers:
Ex.:
	godeploy deploy -f deployment.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		Deploy()
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	//deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	deployCmd.Flags().StringVarP(&deploymentFile, "file", "f", "deployment.yaml", "If the non default deployment file should be used.")
}

func checkConfig() {
	viper.SetConfigName(deploymentFile)
	err := viper.ReadInConfig()
	shared.CheckErr(err, fmt.Sprintf("unable to find deployment file {%v}, Error: %v", deploymentFile, err))

	err = viper.UnmarshalKey("functions", &deploymentDtos)
	shared.CheckErr(err, fmt.Sprintf("unable to parse deployment file {%v}, Error: %v", deploymentFile, err))

	for _, deployment := range deploymentDtos {
		providerNames := shared.Map(deployment.Providers, func(provider shared.Provider) shared.ProviderName { return provider.Name })

		if shared.Contains(providerNames, shared.ProviderAWS) || shared.IsAWSObjectURI(deployment.Archive) { //If necessary should load the AWS credentials
			if credentials.AwsCredentials == nil {
				loadCredentials(shared.AWSCredentialsFile)
				credentials.AwsCredentials = &aws.Credentials{
					AccessKeyID:     viper.GetString(shared.AWSAccessKey),
					SecretAccessKey: viper.GetString(shared.AWSSecretAccessKey),
					SessionToken:    viper.GetString(shared.AWSSessionTokenKey),
				}
			}
		}
		if shared.Contains(providerNames, shared.ProviderGoogle) || shared.IsGoogleObjectURI(deployment.Archive) { //If necessary should load the GCP credentials
			if credentials.GoogleCredentials == nil {
				loadCredentials(shared.GoogleCredentialsFile)
				googleCredentials, err := google2.CredentialsFromJSON(
					context.Background(),
					shared.ReadFile(fmt.Sprintf("%v.%v", shared.GoogleCredentialsFile, shared.DefaultFileExtension)),
					shared.OAuthStorageScope,
					shared.OAuthFunctionScope,
				)
				shared.CheckErr(err, err)
				credentials.GoogleCredentials = googleCredentials
			}
		}
	}
}

func loadCredentials(credentialFile string) {
	viper.SetConfigName(credentialFile)
	err := viper.MergeInConfig()
	shared.CheckErr(err, fmt.Sprintf("unable to find credentials file {%v}, Error: %v", credentialFile, err))
}

func Deploy() {
	var waitGroup sync.WaitGroup
	var deployments []shared.Deployment

	checkConfig() //TODO Rename

	mapDeploymentDtoToDeployment := func(dto shared.DeploymentDto, providerIndex int, regionIndex int) shared.Deployment {
		handlerSplit := strings.Split(dto.Providers[providerIndex].Handler, ".")
		if len(handlerSplit) != 2 {
			fmt.Fprintln(os.Stderr, "Error: unable to parse function handler")
			os.Exit(1)
		}

		return shared.Deployment{
			Archive:         dto.Archive,
			Name:            dto.Name,
			MemorySize:      dto.MemorySize,
			Timeout:         dto.Timeout,
			Runtime:         dto.Providers[providerIndex].Runtime,
			Provider:        dto.Providers[providerIndex].Name,
			HandlerFile:     handlerSplit[0],
			HandlerFunction: handlerSplit[1],
			Region:          dto.Providers[providerIndex].Regions[regionIndex],
		}
	}

	for _, d := range deploymentDtos {
		for i := 0; i < len(d.Providers); i++ {
			for j := 0; j < len(d.Providers[i].Regions); j++ {
				deployments = append(deployments, mapDeploymentDtoToDeployment(d, i, j))
			}
		}
	}

	//TODO Upload Archive before goroutines

	waitGroup.Add(len(deployments))
	for _, deployment := range deployments {
		err := shared.CheckDeployment(deployment)
		shared.CheckErr(err, fmt.Sprintf("deployment check failed, Error: %v\n", err))

		if shared.ProviderAWS == deployment.Provider {
			go my_aws.Deploy(&waitGroup, deployment, credentials)
		}
		if shared.ProviderGoogle == deployment.Provider {
			go google.Deploy(&waitGroup, deployment, credentials)
		}
	}
	waitGroup.Wait()
}
