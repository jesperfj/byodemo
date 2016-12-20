package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/bucket"
	"github.com/jesperfj/byodemo/database"
	"github.com/jesperfj/byodemo/herokuaddon"
)

func finishProvisioning(requestData *herokuaddon.CreateAddonRequest, providerId string) {
	c, err := herokuaddon.NewClientFromCode(clientSecret, requestData.OAuthGrant.Code)
	if err != nil {
		logger.Print(err)
		return
	}
	ownerId, err := c.OwnerId(requestData.Uuid)
	if err != nil {
		c.FailProvisioning(requestData.Uuid)
		logger.Print("Couldn't find owner id for addon: ", requestData.Uuid, " :", err)
		return
	}

	account, err := db.FindAccount(ownerId)
	if err != nil {
		c.FailProvisioning(requestData.Uuid)
		logger.Print("Couldn't provision addon: ", requestData.Uuid, " :", err)
		return
	}

	logger.Print("owner id: ", ownerId)
	logger.Print("account aws id: ", account.AWSAccessKeyId)
	logger.Print(requestData.Region)

	bc, _ := bucket.NewController("us-east-1", account.AWSAccessKeyId, account.AWSSecretAccessKey)
	bucket, err := bc.CreateBucket(providerId)
	if err != nil {
		c.FailProvisioning(requestData.Uuid)
		logger.Print("Couldn't create bucket for addon ", requestData.Uuid, " :", err)
		return
	}

	c.SetAddonConfig(requestData.Uuid, herokuaddon.AddonConfig{
		Config: []herokuaddon.ConfigVar{
			herokuaddon.ConfigVar{
				Name:  "BUCKET_NAME",
				Value: bucket.Name,
			},
			herokuaddon.ConfigVar{
				Name:  "AWS_ACCESS_KEY_ID",
				Value: bucket.AWSAccessKeyId,
			},
			herokuaddon.ConfigVar{
				Name:  "AWS_SECRET_ACCESS_KEY",
				Value: bucket.AWSSecretAccessKey,
			},
		},
	})

	err = db.SaveAddonResource(&database.AddonResource{
		OwnerId:        ownerId,
		ProviderId:     providerId,
		AddonId:        requestData.Uuid,
		AWSAccessKeyId: bucket.AWSAccessKeyId,
	})
	if err != nil {
		c.FailProvisioning(requestData.Uuid)
		logger.Print("Couldn't provision addon: ", requestData.Uuid, " :", err)
		return
	}

	c.CompleteProvisioning(requestData.Uuid)
	logger.Print("Addon provisioning completed for ", requestData.Uuid)
}

func deleteResource(resourceId string) {
	account, addon, err := db.FindAccountForAddon(resourceId)
	if err != nil {
		logger.Print("Cannot complete resource deletion. Error finding account for resource ", resourceId, ": ", err)
		return
	}
	bc, err := bucket.NewController("us-east-1", account.AWSAccessKeyId, account.AWSSecretAccessKey)
	if err != nil {
		logger.Print("Cannot complete resource deletion for ", resourceId, ". Error initializing bucket controller: ", err)
		return
	}
	if bc.DeleteBucket(resourceId, addon.AWSAccessKeyId) {
		logger.Print("Resources deletion complete for ", resourceId)
		err = db.SetDeleted(resourceId)
		if err != nil {
			logger.Print("Resource deletion complete for ", resourceId, " but failed to update database: ", err)
		}
	} else {
		logger.Print("Resource deletion incomplete for ", resourceId, ": ", err)
	}
}

func check(e error) bool {
	if e != nil {
		fmt.Println("Error: " + e.Error())
		return false
	}
	return true
}

var (
	clientSecret = "5dcafd14-8ed0-43c3-9d6e-84a73f8cf9ae"
	logger       = log.New(os.Stderr, "[Web] ", log.Ldate|log.Ltime|log.Lshortfile)
	db           database.DbController
)

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		logger.Fatal("$PORT must be set")
	}

	dbCreds := os.Getenv("DATABASE_URL")
	if dbCreds == "" {
		logger.Fatal("DATABASE_URL must be set")
	}
	dbSecret := os.Getenv("DATABASE_SECRET")
	if dbSecret == "" {
		logger.Fatal("DATABASE_SECRET must be set")
	}
	var err error
	db, err = database.NewController(dbCreds, dbSecret)
	if err != nil {
		logger.Fatal("Error connecting to database: ", err)
	}

	router := gin.New()
	//router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	authorized := router.Group("/addon", gin.BasicAuth(gin.Accounts{"byodemo": "a500bd6fa8f979b469d29bc0c62df367"}))

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	// Utility endpoint to generate a new database key
	// It was easier to put it here than create a new cmd
	router.GET("/genkey", func(c *gin.Context) {
		c.String(http.StatusOK, database.GenerateEncodedKey())
	})

	router.POST("/account", func(c *gin.Context) {
		account := &database.Account{}
		c.Bind(account)
		logger.Print("Saving new AWS creds for Heroku user ", account.OwnerId)
		err := db.SaveAccount(account)
		if err != nil {
			logger.Print("Error saving account: ", err.Error())
			c.String(500, "Error saving account: "+err.Error())
		} else {
			c.String(201, "Account saved.")
		}
	})

	authorized.POST("/heroku/resources", func(c *gin.Context) {
		requestData := &herokuaddon.CreateAddonRequest{}
		c.Bind(requestData)
		fmt.Println("addon id: " + requestData.Uuid)
		fmt.Println(herokuaddon.NewAddonId())
		providerId := herokuaddon.NewAddonId()
		c.JSON(202, herokuaddon.AsyncCreateAddonResponse{
			Id:      providerId,
			Message: "Your addon is being provisioned and will be ready shortly",
		})

		fmt.Println("Response sent")

		go finishProvisioning(requestData, providerId)

	})

	authorized.PUT("/heroku/resources/:id", func(c *gin.Context) {
		data := &herokuaddon.AddonPlanChangeRequest{}
		c.Bind(data)
		c.JSON(200, herokuaddon.AddonPlanChangeResponse{
			Message: "success for " + c.Param("id"),
			Config: map[string]string{
				"URL":   "https://blah.blah.com",
				"OTHER": "blah",
			},
		})
	})

	authorized.DELETE("/heroku/resources/:id", func(c *gin.Context) {
		logger.Print("Deleting addon ", c.Param("id"))
		err := db.MarkResourceForDeletion(c.Param("id"))
		if err != nil {
			c.String(500, err.Error())
		} else {
			c.String(200, "")
		}
		go deleteResource(c.Param("id"))
	})

	router.Run(":" + port)
}
