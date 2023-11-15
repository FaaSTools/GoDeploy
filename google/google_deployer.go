package google

import (
	functions "cloud.google.com/go/functions/apiv1"
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/viper"
	"godeploy/aws"
	"godeploy/shared"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	functions2 "google.golang.org/genproto/googleapis/cloud/functions/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var copyArchiveLock sync.Mutex
var deploymentKey string

var createBucketLock sync.Mutex

var credHolder shared.CredentialsHolder
var deployedFunctions []string

func Deploy(waitGroup *sync.WaitGroup, de shared.Deployment, credentialsHolder shared.CredentialsHolder) {
	defer waitGroup.Done()
	credHolder = credentialsHolder

	storageClient, err := storage.NewClient(context.Background(), option.WithCredentials(credentialsHolder.GoogleCredentials))
	shared.CheckErr(err, fmt.Sprintf("unable to create Google storage client, Error: %v", err))
	defer storageClient.Close()

	functionsClient, err := functions.NewCloudFunctionsClient(context.Background(), option.WithCredentials(credentialsHolder.GoogleCredentials))
	shared.CheckErr(err, fmt.Sprintf("unable to create Google cloud functions client, Error: %v", err))
	defer functionsClient.Close()

	deployedFunctions = getDeployedFunctions(functionsClient)
	// shared.Log(shared.ProviderGoogle, fmt.Sprintf("Deployed functions: %v", deployedFunctions))

	start := time.Now()
	//Check if archive is already present in the storage
	archiveURL := uploadArchive(de.Archive, de.Name, storageClient)
	elapsed := time.Since(start)

	shared.Log(shared.ProviderGoogle, fmt.Sprintf("Location of archive: %v, region: %v, upload took %s", archiveURL, de.Region, elapsed))
	de.Archive = archiveURL

	if shared.Any(deployedFunctions, func(s string) bool { return strings.Contains(s, de.Region) && strings.Contains(s, de.Name) }) {
		updateFunction(de, functionsClient)
	} else {
		createFunction(de, functionsClient)
	}

}

func uploadArchive(archiveURL string, name string, storageClient *storage.Client) string {
	if shared.IsGoogleObjectURI(archiveURL) {
		return archiveURL
	} else if shared.IsAWSObjectURI(archiveURL) {
		copyArchiveLock.Lock()
		if deploymentKey == "" {
			deploymentKey = copyFromAWSToGoogle(archiveURL, storageClient)
		}
		copyArchiveLock.Unlock()

		return buildGoogleUtilURL(shared.ArchiveBucketName, deploymentKey)
	}

	createBucketLock.Lock()
	//Create bucket if it doesn't exist
	bucketHandle := storageClient.Bucket(shared.ArchiveBucketName)

	_, err := bucketHandle.Attrs(context.Background())
	if err != nil {
		if ((strings.Contains(err.Error(), "bucket doesn't exist")) || (strings.Contains(err.Error(), "not exist"))) {
			shared.Log(shared.ProviderGoogle, fmt.Sprintf("Bucket %v doesn't exist, creating new one", shared.ArchiveBucketName))

			err = bucketHandle.Create(context.Background(), viper.GetString(shared.GoogleProjectID), nil)
			createBucketLock.Unlock()
			shared.CheckErr(err, fmt.Sprintf("unable to create bucket on GCP, Error %v", err))
		} else {
			createBucketLock.Unlock()
			shared.CheckErr(err, fmt.Sprintf("unable to access bucket on GCP, Error %v", err))
		}
	} else {
		createBucketLock.Unlock()
	}

	writer := bucketHandle.Object(name).NewWriter(context.Background())
	defer writer.Close()

	f, err := os.Open(archiveURL)
	defer f.Close()
	shared.CheckErr(err, fmt.Sprintf("os.Open: %v, Error: %v", archiveURL, err))

	_, err = io.Copy(writer, f)
	shared.CheckErr(err, fmt.Sprintf("io.Copy: %v", err))

	return buildGoogleUtilURL(shared.ArchiveBucketName, name)
}

func createFunction(d shared.Deployment, functionsClient *functions.CloudFunctionsClient) {
	shared.Log(shared.ProviderGoogle, fmt.Sprintf("Started creating function %v in region %v with %v MB memory", d.Name, d.Region, d.MemorySize))

	start := time.Now()
	projectID := viper.GetString(shared.GoogleProjectID)
	sourceArchive := &functions2.CloudFunction_SourceArchiveUrl{SourceArchiveUrl: d.Archive}
	timeout := &durationpb.Duration{
		Seconds: int64(d.Timeout),
		Nanos:   0,
	}

	functionName := fmt.Sprintf("projects/%v/locations/%v/functions/%v", projectID, d.Region, d.Name)
	function := functions2.CloudFunction{
		Name:              functionName,
		SourceCode:        sourceArchive,
		Trigger:           &functions2.CloudFunction_HttpsTrigger{},
		Status:            0,
		EntryPoint:        d.HandlerFunction,
		Runtime:           d.Runtime,
		Timeout:           timeout,
		AvailableMemoryMb: d.MemorySize,
		MaxInstances:      shared.DefaultMaxFunctionInstances,
	}
	location := fmt.Sprintf("projects/%v/locations/%v", projectID, d.Region)
	request := functions2.CreateFunctionRequest{
		Location: location,
		Function: &function,
	}
	createFunctionOperation, err := functionsClient.CreateFunction(context.Background(), &request)
	shared.CheckErr(err, fmt.Sprintf("unable to create function, Error: %v", err))

	poll, err := createFunctionOperation.Wait(context.Background())
	shared.CheckErr(err, fmt.Sprintf("unable to wait for function deployment, Error: %v", err))
	
	elapsed := time.Since(start)

	shared.Log(shared.ProviderGoogle, fmt.Sprintf("Finished creating function %v in region %v with %v MB memory, took %s", poll.Name, d.Region, d.MemorySize, elapsed))
}

func updateFunction(d shared.Deployment, functionsClient *functions.CloudFunctionsClient) {
	shared.Log(shared.ProviderGoogle, fmt.Sprintf("Started updating function %v in region %v with %v MB memory", d.Name, d.Region, d.MemorySize))

	sourceArchive := &functions2.CloudFunction_SourceArchiveUrl{SourceArchiveUrl: d.Archive}
	timeout := &durationpb.Duration{
		Seconds: int64(d.Timeout),
		Nanos:   0,
	}
	functionName := fmt.Sprintf("projects/%v/locations/%v/functions/%v", viper.GetString(shared.GoogleProjectID), d.Region, d.Name)
	function := &functions2.CloudFunction{
		Name:              functionName,
		SourceCode:        sourceArchive,
		Trigger:           &functions2.CloudFunction_HttpsTrigger{},
		Status:            0,
		EntryPoint:        d.HandlerFunction,
		Runtime:           d.Runtime,
		Timeout:           timeout,
		AvailableMemoryMb: d.MemorySize,
		MaxInstances:      shared.DefaultMaxFunctionInstances,
	}
	updateFunctionRequest := &functions2.UpdateFunctionRequest{
		Function: function,
	}
	updateFunctionOperation, err := functionsClient.UpdateFunction(context.Background(), updateFunctionRequest)
	shared.CheckErr(err, fmt.Sprintf("unable to update updateFunctionOperation, Error: %v", err))

	poll, err := updateFunctionOperation.Wait(context.Background())
	shared.CheckErr(err, fmt.Sprintf("unable to wait for function deployment, Error: %v", err))

	shared.Log(shared.ProviderGoogle, fmt.Sprintf("Finished updating function %v in region %v with %v MB memory", poll.Name, d.Region, d.MemorySize))
}

func getDeployedFunctions(functionsClient *functions.CloudFunctionsClient) []string {
	var f []string

	listFunctions := functionsClient.ListFunctions(context.Background(), &functions2.ListFunctionsRequest{Parent: fmt.Sprintf("projects/%v/locations/-", viper.GetString(shared.GoogleProjectID))})
	for {
		item, err := listFunctions.Next()
		if err == iterator.Done {
			break
		}
		shared.CheckErr(err, err)
		f = append(f, item.Name)
	}
	return f
}

//Helper function
func buildGoogleUtilURL(bucket string, name string) string {
	return fmt.Sprintf("gs://%v/%v", bucket, name)
}

func isAWSStorageURL(url string) bool {
	return strings.HasPrefix(url, "https://") && strings.Contains(url, "s3.amazonaws.com/")
}

func copyFromAWSToGoogle(srcURL string, storageClient *storage.Client) string {
	bucket, key := shared.ParseStorageObjectURI(srcURL)
	if bucket == "" && key == "" {
		fmt.Fprintln(os.Stderr, "Error:", fmt.Sprintf("unable to parse S3 object URI {%v}", srcURL))
		os.Exit(1)
	}
	fmt.Printf("Bucket: %v, Key: %v\n", bucket, key)

	cfg := aws.SetupConfig(shared.DefaultAWSRegion, credHolder)
	s3Client := s3.NewFromConfig(cfg)

	getObjectInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	object, err := s3Client.GetObject(context.Background(), getObjectInput)
	shared.CheckErr(err, fmt.Sprintf("unable to get object from S3, Error: %v", err))
	defer object.Body.Close()

	//Create bucket if it doesn't exist
	bucketHandle := storageClient.Bucket(shared.ArchiveBucketName)
	_, err = bucketHandle.Attrs(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "bucket doesn't exist") {
			fmt.Println("Bucket doesn't exist, creating new one")
			if err = bucketHandle.Create(context.Background(), viper.GetString(shared.GoogleProjectID), nil); err != nil {
				shared.CheckErr(err, fmt.Sprintf("unable to create bucket on GCP, Error %v", err))
			}
		} else {
			shared.CheckErr(err, fmt.Sprintf("unable to access bucket on GCP, Error %v", err))
		}
	}

	writer := bucketHandle.Object(key).NewWriter(context.Background())

	if _, err = io.Copy(writer, object.Body); err != nil {
		log.Fatalf("io.Copy: %v", err)
	}

	if err = writer.Close(); err != nil {
		log.Fatalf("Writer.Close: %v", err)
	}

	return key
}
