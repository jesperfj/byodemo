package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/bucket"
	"github.com/jesperfj/byodemo/database"
	"github.com/jesperfj/byodemo/heroku"
)

func finishProvisioning(requestData *heroku.CreateAddonRequest, providerId string) {
	c, err := heroku.NewClientFromCode(config.clientSecret, requestData.OAuthGrant.Code)
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

	c.SetAddonConfig(requestData.Uuid, heroku.AddonConfig{
		Config: []heroku.ConfigVar{
			heroku.ConfigVar{
				Name:  "BUCKET_NAME",
				Value: bucket.Name,
			},
			heroku.ConfigVar{
				Name:  "AWS_ACCESS_KEY_ID",
				Value: bucket.AWSAccessKeyId,
			},
			heroku.ConfigVar{
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

func setupAddonRoutes(router *gin.Engine) {

	addon := router.Group("/addon", gin.BasicAuth(gin.Accounts{"byodemo": config.addonProviderToken}))

	addon.POST("/heroku/resources", func(c *gin.Context) {
		requestData := &heroku.CreateAddonRequest{}
		c.Bind(requestData)
		providerId := heroku.NewAddonId()
		c.JSON(202, heroku.AsyncCreateAddonResponse{
			Id: providerId,
			Message: "Warning: This request will fail silently if AWS credentials have not been configured.\n" +
				"Set up AWS credentials for your team at https://byodemo-addon.herokuapp.com",
		})

		go finishProvisioning(requestData, providerId)

	})

	addon.PUT("/heroku/resources/:id", func(c *gin.Context) {
		data := &heroku.AddonPlanChangeRequest{}
		c.Bind(data)
		c.JSON(200, heroku.AddonPlanChangeResponse{
			Message: "success for " + c.Param("id"),
			Config: map[string]string{
				"URL":   "https://blah.blah.com",
				"OTHER": "blah",
			},
		})
	})

	addon.DELETE("/heroku/resources/:id", func(c *gin.Context) {
		logger.Print("Deleting addon ", c.Param("id"))
		err := db.MarkResourceForDeletion(c.Param("id"))
		if err != nil {
			c.String(500, err.Error())
		} else {
			c.String(200, "")
		}
		go deleteResource(c.Param("id"))
	})

}
