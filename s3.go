package s3

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
)

type S3AddonController struct {
	session session.Session
}

var (
	logger = log.New(os.Stderr, "[s3] ", log.Ldate|log.Ltime|log.Lshortfile)
)

func New(region string, awsAccessKeyId string, awsSecretAccessKey string) string {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(awsAccessKeyId, awsSecretAccessKey, nil),
	})
	if err != nil {
		logger.Print("Error initializing S3 controller: ", err.Error())
		return err
	}
	return S3AddonController{session: sess}
}
