package shared

//Default file extension (for configuration file)
const DefaultFileExtension = "yaml"

//Default regions
const DefaultAWSRegion = "us-east-1"
const DefaultGoogleRegion = "us-east1"

//Default serverless function roles
const DefaultAWSRole = "LabRole"

//Keys needed for parsing credentials from aws-credentials.yaml
const AWSAccessKey = "aws_access_key_id"
const AWSSecretAccessKey = "aws_secret_access_key"
const AWSSessionTokenKey = "aws_session_token"
const AWSRoleKey = "role"

//Constants
const ArchiveBucketName = "godeploy-deployments"
const GoogleProjectID = "project_id"
const AWSCredentialsFile = "aws-credentials"
const GoogleCredentialsFile = "google-credentials"
const OAuthStorageScope = "https://www.googleapis.com/auth/devstorage.full_control"
const OAuthFunctionScope = "https://www.googleapis.com/auth/cloud-platform"
const DefaultMaxFunctionInstances = 5
