package shared

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"golang.org/x/oauth2/google"
)

type CredentialsHolder struct {
	AwsCredentials    *aws.Credentials
	GoogleCredentials *google.Credentials
}
