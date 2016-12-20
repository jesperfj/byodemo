package bucket

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
)

type BucketController struct {
	session *session.Session
	s3svc   *s3.S3
	iamsvc  *iam.IAM
}

type Bucket struct {
	Name               string
	Region             string
	UserName           string
	UserARN            string
	AWSAccessKeyId     string
	AWSSecretAccessKey string
}

const (
	policyDocTemplate = `{
  "Id": "Policy%s",
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Stmt%s",
      "Action": [
        "s3:AbortMultipartUpload",
        "s3:DeleteObject",
        "s3:DeleteObjectVersion",
        "s3:GetAccelerateConfiguration",
        "s3:GetBucketAcl",
        "s3:GetBucketCORS",
        "s3:GetBucketLocation",
        "s3:GetBucketLogging",
        "s3:GetBucketNotification",
        "s3:GetBucketVersioning",
        "s3:GetBucketWebsite",
        "s3:GetLifecycleConfiguration",
        "s3:GetObject",
        "s3:GetObjectAcl",
        "s3:GetObjectTorrent",
        "s3:GetObjectVersion",
        "s3:GetObjectVersionAcl",
        "s3:GetObjectVersionTorrent",
        "s3:GetReplicationConfiguration",
        "s3:ListBucket",
        "s3:ListBucketMultipartUploads",
        "s3:ListBucketVersions",
        "s3:ListMultipartUploadParts",
        "s3:PutAccelerateConfiguration",
        "s3:PutBucketAcl",
        "s3:PutBucketCORS",
        "s3:PutBucketLogging",
        "s3:PutBucketNotification",
        "s3:PutBucketRequestPayment",
        "s3:PutBucketTagging",
        "s3:PutBucketVersioning",
        "s3:PutBucketWebsite",
        "s3:PutLifecycleConfiguration",
        "s3:PutReplicationConfiguration",
        "s3:PutObject",
        "s3:PutObjectAcl",
        "s3:PutObjectVersionAcl",
        "s3:ReplicateDelete",
        "s3:ReplicateObject",
        "s3:RestoreObject"
      ],
      "Effect": "Allow",
      "Resource": [
        "arn:aws:s3:::%s", 
        "arn:aws:s3:::%s/*"
      ],
      "Principal": {
        "AWS": [
          "%s"
        ]
      }
    }
  ]
}`
)

var (
	logger = log.New(os.Stderr, "[bucket] ", log.Ldate|log.Ltime|log.Lshortfile)
)

func NewController(region string, awsAccessKeyId string, awsSecretAccessKey string) (BucketController, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(awsAccessKeyId, awsSecretAccessKey, ""),
	})
	if err != nil {
		logger.Print("Error initializing bucket controller: ", err.Error())
		return BucketController{}, err
	}
	return BucketController{session: sess, s3svc: s3.New(sess), iamsvc: iam.New(sess)}, nil
}

func (c *BucketController) CreateBucket(providerId string) (bucket Bucket, err error) {

	// Create the S3 bucket

	bucket.Name = "bucket-" + providerId
	_, err = c.s3svc.CreateBucket(&s3.CreateBucketInput{Bucket: &bucket.Name})
	if err != nil {
		logger.Print("Error creating bucket: ", err)
		return bucket, err
	}

	// Create the IAM user that will access the bucket

	bucket.UserName = "user-" + providerId
	createUserOutput, err := c.iamsvc.CreateUser(&iam.CreateUserInput{UserName: &bucket.UserName})
	if err != nil {
		logger.Print("Error creating IAM User: ", err)
		return bucket, err
	}
	bucket.UserARN = *createUserOutput.User.Arn
	logger.Print("Created IAM User ", bucket.UserARN)

	// Create the credentials for the IAM user

	credResp, err := c.iamsvc.CreateAccessKey(&iam.CreateAccessKeyInput{UserName: &bucket.UserName})
	if err != nil {
		logger.Print("Error creating access keys for IAM User: ", err)
		return bucket, err
	}

	bucket.AWSAccessKeyId = *credResp.AccessKey.AccessKeyId
	bucket.AWSSecretAccessKey = *credResp.AccessKey.SecretAccessKey

	logger.Print("Created access key ", bucket.AWSAccessKeyId, " for IAM user ", bucket.UserARN)

	policyDoc := fmt.Sprintf(policyDocTemplate, providerId, providerId, bucket.Name, bucket.Name, bucket.UserARN)

	// Setting bucket policy that refers to a newly created IAM user can fail seemingly due to an AWS race condition.
	// Adding a 3 second delay seems to remove this issue

	time.Sleep(3 * time.Second)

	// ...but just in case we will retry 5 times with 3 second delays between them.

	for i := 0; ; i++ {
		_, err = c.s3svc.PutBucketPolicy(&s3.PutBucketPolicyInput{
			Bucket: &bucket.Name,
			Policy: &policyDoc,
		})
		if err != nil {
			logger.Print("Error setting bucket policy: ", err)
			if i < 5 {
				logger.Print("Retrying after a delay...")
				time.Sleep(3 * time.Second)
			} else {
				logger.Print(bucket.Name, ": Failed to set bucket policy after ", i, " retries. Giving up.")
				return bucket, err
			}
		} else {
			logger.Print("Bucket policy set for ", bucket.Name)
			break
		}
	}
	return bucket, nil
}

func (c *BucketController) DeleteBucket(providerId string, awsAccessKeyId string) bool {

	success := true
	// First, delete all objects in bucket
	c.DeleteAllObjects(providerId)
	// if delete all objects fail, errors have already been logged and we'll keep going.

	bucketName := "bucket-" + providerId
	_, err := c.s3svc.DeleteBucket(&s3.DeleteBucketInput{Bucket: &bucketName})
	if err != nil {
		logger.Print("Error deleting bucket: ", err)
		success = false
		// keep going
	}
	userName := "user-" + providerId

	_, err = c.iamsvc.DeleteAccessKey(&iam.DeleteAccessKeyInput{
		AccessKeyId: &awsAccessKeyId,
		UserName:    &userName,
	})
	if err != nil {
		logger.Print("Error deleting IAM User Access Key: ", err)
		success = false
		// keep going
	}

	_, err = c.iamsvc.DeleteUser(&iam.DeleteUserInput{UserName: &userName})
	if err != nil {
		logger.Print("Error deleting IAM user: ", err)
		success = false
		// keep going
	}

	return success
}

func (c *BucketController) DeleteAllObjects(providerId string) (err error) {
	bucketName := "bucket-" + providerId
	for {
		output, err := c.s3svc.ListObjects(&s3.ListObjectsInput{Bucket: &bucketName})
		if err != nil {
			logger.Print("Error listing objects for bucket ", bucketName, ": ", err)
			return err
		}
		if len(output.Contents) == 0 {
			logger.Print("No more objects to delete from ", bucketName)
			return nil
		}
		deleteList := make([]*s3.ObjectIdentifier, len(output.Contents))
		for i, obj := range output.Contents {
			deleteList[i] = &s3.ObjectIdentifier{Key: obj.Key}
		}
		_, err = c.s3svc.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3.Delete{Objects: deleteList},
		})
		if err != nil {
			logger.Print("Error deleting objects from ", bucketName, ": ", err)
			return err
		}
		logger.Print("Deleted ", len(deleteList), " objects from ", bucketName)
	}
}
