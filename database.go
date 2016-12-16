package database

import (
//	"github.com/fernet/fernet-go"
)

type Account struct {
	OwnerId            string
	AWSAccessKeyId     string
	AWSSecretAccessKey string
}

type AddonResource struct {
	OwnerId    string
	ProviderId string
}

func FindAccount(ownerUuid string) (Account, error) {
	return Account{
		AWSAccessKeyId:     "AKIAJ37GNBGTT63ZQJIA",
		AWSSecretAccessKey: "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",
	}, nil
}

func FindAccountForAddon(providerId string) (Account, error) {
	return FindAccount("whatever")
}
