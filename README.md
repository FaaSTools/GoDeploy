# GoDeploy

## Info

`GoDeploy` is a CLI tool written in **Go** that aims to simplify the process of deploying serverless functions to multiple
FaaS providers, i.e., AWS Lambda and Google Cloud Functions.

## Requirements

_aws-credentials.yaml:_

````yaml
role: "<FUNCTION_USER_GROUP>"
aws_access_key_id: "<ACCESS_KEY_ID>"
aws_secret_access_key: "<SECRET_ACCESS_KEY>"
aws_session_token: "<SESSION_TOKEN>"
````

Info: When using this library in combination with the _AWSAcademy_ course, **role** will most likely be _LabRole_.

_gcp-credentials.yaml:_

````yaml
{
  "type": "service_account",
  "project_id": "<PROJECT_ID>",
  "private_key_id": "<PRIVATE_KEY_ID>",
  "private_key": "-----BEGIN PRIVATE KEY-----<PRIVATE_KEY>-----END PRIVATE KEY-----\n"
}
````

For more information how to retrieve the information needed for this file, see: [Google Cloud](https://cloud.google.com/iam/docs/creating-managing-service-accounts)


## How To Use

1. Install Go, for more information, see https://go.dev/doc/install
2. Clone the repository to a local folder
3. run `go install` inside the root directory of the project
4. Find the `godeploy` executable inside the `/go/bin` directory
5. Run the deployment with the following command `godeploy deploy` (If your deployment file's name differs from **deployment.yaml** specify the file with the `-f` parameter)

## Project Structure

The structure of the archive (.zip) for the project using *GoDeploy* should look something like this.

``` shell
.
├── aws-credentials.yaml
├── gcp-credentials.yaml
├── code
│   ├── ...
```

You furthermore need a deployment file, like `deployment.yaml`, that describes where the functions should be deployed.


# Example

See [here](examples/deployment.yaml).