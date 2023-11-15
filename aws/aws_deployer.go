package aws

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	types2 "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/viper"
	"godeploy/shared"
	"google.golang.org/api/option"
	"os"
	"strings"
	"sync"
	"time"
)

//Map that stores the already created buckets
var bucketExistsMap = make(map[string]string)

var credHolder shared.CredentialsHolder

func Deploy(waitGroup *sync.WaitGroup, d shared.Deployment, credentialsHolder shared.CredentialsHolder) {
	defer waitGroup.Done()
	credHolder = credentialsHolder
	cfg := SetupConfig(d.Region, credHolder)
	lambdaClient := lambda.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	start := time.Now()
	bucketName, objectKey := uploadArchive(s3Client, d.Region, d.Name, d.Archive)
	elapsed := time.Since(start)
	
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Name: %v, Region: %v, upload took %s", d.Name, d.Region, elapsed))

	d.Bucket = bucketName
	d.Key = objectKey

	r := getRoleARN(iam.NewFromConfig(cfg))
	createFunction(lambdaClient, d, r)
}

func uploadArchive(client *s3.Client, region string, objectKey string, archiveURL string) (string, string) {
	//Check if bucket exists for a specific region
	bucketName := bucketExists(client, region)
	if bucketName == "" {
		bucketName = createBucket(client, region)
	} else {
		shared.Log(shared.ProviderAWS, fmt.Sprintf("deployment bucket for region %v already exists", region))
	}

	if shared.IsAWSObjectURI(archiveURL) {
		return archiveURL, " "
	} else if shared.IsGoogleObjectURI(archiveURL) {
		return copyFromGoogleToAWS(archiveURL, bucketName, client)
	}

	f, err := os.Open(archiveURL)
	shared.CheckErr(err, fmt.Sprintf("os.Open: %v, Error: %v", archiveURL, err))
	defer f.Close()

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: &bucketName, Key: &objectKey, Body: f})
	shared.CheckErr(err, fmt.Sprintf("unable to upload archive to bucket on AWS, Error: %v", err))

	return bucketName, objectKey
}

func createBucket(storageClient *s3.Client, region string) string {
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Create bucket for region %v", region))

	bucketName := shared.ArchiveBucketName + "-" + region
	bucketInput := &s3.CreateBucketInput{Bucket: &bucketName}
	//Not default locations (other than "us-east-1") need an explicit LocationConstraint set
	if region != shared.DefaultAWSRegion {
		bucketInput.CreateBucketConfiguration = &types2.CreateBucketConfiguration{LocationConstraint: types2.BucketLocationConstraint(region)}
	}
	_, err := storageClient.CreateBucket(context.Background(), bucketInput)
	shared.CheckErr(err, fmt.Sprintf("unable to create bucket on AWS for region %v, Error: %v", region, err))

	return bucketName
}

//Check if the bucket containing the deployments already exists for the specified region
func bucketExists(client *s3.Client, region string) string {
	if bucketExistsMap[region] != "" {
		shared.Log(shared.ProviderAWS, fmt.Sprintf("Already checked if bucket exists for region %v", region))
		return bucketExistsMap[region]
	}

	output, err := client.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	shared.CheckErr(err, fmt.Sprintf("unable to list buckets on AWS, error msg: %v", err))

	var bucketNames []string
	for _, b := range output.Buckets {
		if b.Name != nil {
			bucketNames = append(bucketNames, *b.Name)
		}
	}
	for _, bName := range bucketNames {
		if strings.Contains(bName, region) {
			bucketExistsMap[region] = bName
			return bName
		}
	}
	return ""
}

func SetupConfig(region string, c shared.CredentialsHolder) aws.Config {
	staticCredentialsProvider := credentials.StaticCredentialsProvider{Value: *c.AwsCredentials}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region), config.WithCredentialsProvider(staticCredentialsProvider))
	shared.CheckErr(err, fmt.Sprintf("unable to load AWS SDK config, Error: %v", err))

	return cfg
}

func createFunction(client *lambda.Client, d shared.Deployment, role string) string {
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Started creating function %v in region %v with %v MB memory", d.Name, d.Region, d.MemorySize))
	
	start := time.Now()
	handler := d.HandlerFile

	if strings.Contains(d.Runtime, "python") {
		handler = fmt.Sprintf("%v.%v", d.HandlerFile, d.HandlerFunction)
	}

	params := &lambda.CreateFunctionInput{
		Code:         &types.FunctionCode{S3Bucket: &d.Bucket, S3Key: &d.Key},
		FunctionName: &d.Name,
		Role:         &role,
		Handler:      &handler,
		Timeout:      &d.Timeout,
		MemorySize:   &d.MemorySize,
		Runtime:      types.Runtime(d.Runtime),
		PackageType:  types.PackageTypeZip,
	}

	createdFunction, err := client.CreateFunction(context.Background(), params)

	if err != nil && strings.Contains(err.Error(), "https response error StatusCode: 409") && 
		strings.Contains(err.Error(), "ResourceConflictException: Function already exist") {
		shared.Log(shared.ProviderAWS, fmt.Sprintf("Function %v in region %v already exists. Updating function...", d.Name, d.Region))
		return updateFunction(client, d, role, start)
	} else {
		shared.CheckErr(err, fmt.Sprintf("Error: unable to create function %v, Error %v", *params.FunctionName, err))
	}

	elapsed := time.Since(start)
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Finished creating function %v in region %v with %v MB memory, took %s", d.Name, d.Region, d.MemorySize, elapsed))
	return *createdFunction.FunctionArn
}

func updateFunction(client *lambda.Client, d shared.Deployment, role string, start time.Time) string {
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Started updating function %v in region %v with %v MB memory", d.Name, d.Region, d.MemorySize))

	configurationParams := &lambda.UpdateFunctionConfigurationInput{
		FunctionName: &d.Name,
		Handler:      &d.HandlerFile,
		Timeout:      &d.Timeout,
		MemorySize:   &d.MemorySize,
		Role:         &role,
		Runtime:      types.Runtime(d.Runtime),
	}
	updatedFunction, err := client.UpdateFunctionConfiguration(context.Background(), configurationParams)
	shared.CheckErr(err, fmt.Sprintf("unable to update function configuration, Error: %v", err))

	maxRetries := 5
	retryDelay := 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
        _, err = client.UpdateFunctionCode(context.Background(), &lambda.UpdateFunctionCodeInput{
			FunctionName: &d.Name,
			S3Bucket:     &d.Bucket,
			S3Key:        &d.Key,
		})
		if err != nil && strings.Contains(err.Error(), "ResourceConflictException: The operation cannot be performed at this time. An update is in progress for resource") {
			shared.Log(shared.ProviderAWS, fmt.Sprintf("Config-update in progress, retrying updating code in %v... (Attempt %d/%d)", retryDelay, i+1, maxRetries))
			time.Sleep(retryDelay)
			continue
		} else {
			break
		}
    }

	shared.CheckErr(err, fmt.Sprintf("unable to update function code, Error: %v", err))
	elapsed := time.Since(start)
	shared.Log(shared.ProviderAWS, fmt.Sprintf("Finished updating function %v in region %v with %v MB memory, took %s", d.Name, d.Region, d.MemorySize, elapsed))

	return *updatedFunction.FunctionArn
}

func getDeployedFunctions(c *lambda.Client) *lambda.ListFunctionsOutput {
	functions, err := c.ListFunctions(context.Background(), nil)
	shared.CheckErr(err, fmt.Sprintf("unable to list deployed functions, Error: %v", err))
	return functions
}

func getFunctionNames(f lambda.ListFunctionsOutput) []string {
	functionNameMapper := func(f types.FunctionConfiguration) string { return *f.FunctionName }
	return shared.Map(f.Functions, functionNameMapper)
}

func getRoleARN(c *iam.Client) string {
	role := viper.GetString(shared.AWSRoleKey)
	r, err := c.GetRole(context.Background(), &iam.GetRoleInput{RoleName: &role})
	shared.CheckErr(err, fmt.Sprintf("unable to get role ARN for role name {%v}, Error: %v", role, err))
	return *r.Role.Arn
}

func copyFromGoogleToAWS(srcURL string, targetBucket string, s3Client *s3.Client) (string, string) {
	storageClient, err := storage.NewClient(context.Background(), option.WithCredentials(credHolder.GoogleCredentials))
	shared.CheckErr(err, fmt.Sprintf("unable to create Google storage client, Error: %v", err))
	defer storageClient.Close()

	bucket, key := shared.ParseStorageObjectURI(srcURL)
	if bucket == "" && key == "" {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Error: unable to parse Google object URI {%v}", srcURL))
		os.Exit(1)
	}

	reader, err := storageClient.Bucket(bucket).Object(key).NewReader(context.Background())
	shared.CheckErr(err, fmt.Sprintf("unable to read from Google object, Error: %v", err))
	defer reader.Close()

	_, err = s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        &targetBucket,
		Key:           &key,
		Body:          reader,
		ContentLength: reader.Attrs.Size,
	})
	shared.CheckErr(err, fmt.Sprintf("unable to put object in S3, Error: %v\n", err))

	return targetBucket, key
}

func buildS3URI(bucket string, key string) string {
	return fmt.Sprintf("s3://%v/%v", bucket, key)
}
