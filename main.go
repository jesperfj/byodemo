package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/accounts"
	"github.com/jesperfj/byodemo/herokuaddon"
)

func finishProvisioning(requestData *herokuaddon.CreateAddonRequest, providerId string) {
	c, err := herokuaddon.NewClientFromCode(clientSecret, requestData.OAuthGrant.Code)
	if err != nil {
		logger.Print(err)
		return
	}
	ownerId, _ := c.OwnerId(requestData.Uuid)

	_, err = accounts.FindAccount(ownerId)
	if err != nil {
		c.FailProvisioning(requestData.Uuid)
		logger.Print("Couldn't provision addon: ", requestData.Uuid)
		return
	}

	logger.Print("owner id: ", ownerId)

	c.SetAddonConfig(requestData.Uuid, herokuaddon.AddonConfig{
		Config: []herokuaddon.ConfigVar{
			herokuaddon.ConfigVar{
				Name:  "URL",
				Value: "https://blahblah.com",
			},
			herokuaddon.ConfigVar{
				Name:  "OTHER",
				Value: ownerId,
			},
		},
	})

	c.CompleteProvisioning(requestData.Uuid)
	logger.Print("Addon provisioning completed for ", requestData.Uuid)
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
)

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		logger.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	authorized := router.Group("/addon", gin.BasicAuth(gin.Accounts{"byodemo": "a500bd6fa8f979b469d29bc0c62df367"}))

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "")
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
		fmt.Println(data)
		fmt.Println(data.AppId)
		c.JSON(200, herokuaddon.AddonPlanChangeResponse{
			Message: "success for " + c.Param("id"),
			Config: map[string]string{
				"URL":   "https://blah.blah.com",
				"OTHER": "blah",
			},
		})
	})

	authorized.DELETE("/heroku/resources/:id", func(c *gin.Context) {
	})

	router.Run(":" + port)
}
