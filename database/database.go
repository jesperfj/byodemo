package database

import (
	"errors"
	"log"
	"os"

	"database/sql"
	"github.com/lib/pq"
	_ "github.com/lib/pq"

	fernet "github.com/fernet/fernet-go"
)

type DbController struct {
	db        *sql.DB
	fernetKey *fernet.Key
}

type Account struct {
	OwnerId            string `json:"owner_id"`
	AWSAccessKeyId     string `json:"aws_access_key_id"`
	AWSSecretAccessKey string `json:"aws_secret_access_key"`
}

type AddonResource struct {
	OwnerId        string
	ProviderId     string
	AddonId        string
	AWSAccessKeyId string
}

var (
	logger = log.New(os.Stderr, "[db] ", log.Ldate|log.Ltime|log.Lshortfile)
)

func NewController(creds string, secret string) (DbController, error) {
	c := DbController{}
	db, err := sql.Open("postgres", creds)
	if err != nil {
		return c, err
	}
	if err := db.Ping(); err != nil {
		logger.Print("Problem connecting to database: ", err)
		return c, err
	}
	c.db = db

	key, err := fernet.DecodeKey(secret)
	if err != nil {
		logger.Print("Couldn't parse secret key. Is it a correctly formatted fernet key? Error: ", err)
		return c, err
	}
	c.fernetKey = key

	logger.Print("Successfully connected to database")
	return c, nil
}

func GenerateEncodedKey() string {
	key := fernet.Key{}
	key.Generate()
	return key.Encode()
}

// TODO: This and function below needs to be deduped a bit
func (c *DbController) FindAccount(ownerUuid string) (Account, error) {
	rows, err := c.db.Query("SELECT owner_uuid, aws_access_key_id, aws_secret_access_key_token FROM accounts WHERE owner_uuid = $1", ownerUuid)
	if err != nil {
		logger.Print("Error querying database for account: ", err)
		return Account{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		logger.Print("Account for ", ownerUuid+" not found in database")
		return Account{}, errors.New("Account not found")
	}
	var uuid string
	var key string
	var encryptedSecret []byte
	if err := rows.Scan(&uuid, &key, &encryptedSecret); err != nil {
		log.Print("Error reading database row: ", err)
		return Account{}, err
	}
	decryptedSecret := fernet.VerifyAndDecrypt(encryptedSecret, -1, []*fernet.Key{c.fernetKey})
	return Account{
		OwnerId:            uuid,
		AWSAccessKeyId:     key,
		AWSSecretAccessKey: string(decryptedSecret),
	}, nil
}

func (c *DbController) FindAccounts(ownerIds []string) map[string]string {
	rows, _ := c.db.Query(`
		 SELECT owner_uuid, aws_access_key_id
		 FROM   accounts
		 WHERE  owner_uuid = ANY($1)
		`, pq.Array(ownerIds))
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var owner string
		var key string
		rows.Scan(&owner, &key)
		result[owner] = key
	}
	return result
}

func (c *DbController) FindAccountForAddon(providerId string) (account Account, addon AddonResource, err error) {
	rows, err := c.db.Query(`
		 SELECT a.owner_uuid, a.aws_access_key_id, a.aws_secret_access_key_token,
		        ar.owner_uuid, ar.provider_resource_id, ar.heroku_resource_id, ar.aws_access_key_id
		 FROM   accounts a, addon_resources ar 
		 WHERE  a.owner_uuid = ar.owner_uuid
		   AND  ar.provider_resource_id = $1
		`,
		providerId)
	if err != nil {
		logger.Print("Error querying database for account: ", err)
		return account, addon, err
	}
	defer rows.Close()
	if !rows.Next() {
		logger.Print("Account for provider id ", providerId+" not found in database")
		return account, addon, errors.New("Account not found")
	}
	//var uuid string
	//var key string
	var encryptedSecret []byte
	if err := rows.Scan(
		&account.OwnerId, &account.AWSAccessKeyId, &encryptedSecret,
		&addon.OwnerId, &addon.ProviderId, &addon.AddonId, &addon.AWSAccessKeyId); err != nil {
		log.Print("Error reading database row: ", err)
		return account, addon, err
	}
	account.AWSSecretAccessKey = string(fernet.VerifyAndDecrypt(encryptedSecret, -1, []*fernet.Key{c.fernetKey}))
	return account, addon, nil
}

func (c *DbController) SaveAddonResource(newAddonResource *AddonResource) error {
	_, err := c.db.Exec(
		`INSERT INTO addon_resources (owner_uuid, provider_resource_id, heroku_resource_id, aws_access_key_id) 
		 VALUES ($1,$2,$3,$4)`,
		newAddonResource.OwnerId, newAddonResource.ProviderId, newAddonResource.AddonId, newAddonResource.AWSAccessKeyId)
	if err != nil {
		logger.Print("Error saving addon resource: ", err)
		return err
	}
	return nil
}

func (c *DbController) SaveAccount(newAccount *Account) error {
	encrypted, err := fernet.EncryptAndSign([]byte(newAccount.AWSSecretAccessKey), c.fernetKey)
	if err != nil {
		logger.Print("Error encrypting secret access key: ", err)
		return err
	}
	_, err = c.db.Exec(
		`INSERT INTO accounts (owner_uuid, aws_access_key_id, aws_secret_access_key_token)
		 VALUES ($1,$2,$3)`,
		newAccount.OwnerId, newAccount.AWSAccessKeyId, encrypted)
	if err != nil {
		logger.Print("Error saving account: ", err)
		return err
	}
	return nil
}

func (c *DbController) DeleteAccount(ownerId string) error {
	result, err := c.db.Exec(
		`DELETE FROM accounts WHERE owner_uuid = $1`, ownerId)
	if err != nil {
		logger.Print("Error deleting account: ", err)
		return err
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected != 1 {
		logger.Print("While deleting account for owner id ", ownerId, ", ", rowsAffected, " was affected. 1 row was expected.")
	}
	return nil
}

func (c *DbController) MarkResourceForDeletion(providerId string) error {
	result, err := c.db.Exec(
		"UPDATE addon_resources SET mark_for_deletion=true WHERE provider_resource_id = $1",
		providerId)
	if err != nil {
		logger.Print("Error marking resource deleted : ", err)
		return err
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected != 1 {
		logger.Print("While marking resource ", providerId, " for deletion, ", rowsAffected, " was affected. 1 row was expected.")
	}
	return nil
}

func (c *DbController) SetDeleted(providerId string) error {
	result, err := c.db.Exec(
		"UPDATE addon_resources SET deleted_at=now() WHERE provider_resource_id = $1",
		providerId)
	if err != nil {
		logger.Print("Error updating resource as deleted : ", err)
		return err
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected != 1 {
		logger.Print("While updating resource ", providerId, " as deleted, ", rowsAffected, " was affected. 1 row was expected.")
	}
	return nil
}
