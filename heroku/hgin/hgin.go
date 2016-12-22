package hgin

import (
	"encoding/base64"
	"log"
	"net/url"
	"os"

	fernet "github.com/fernet/fernet-go"
	"github.com/gin-gonic/gin"
	"github.com/jesperfj/byodemo/heroku"
)

const (
	CookieName = "heroku"
)

var (
	logger = log.New(os.Stderr, "[hgin] ", log.Ldate|log.Ltime|log.Lshortfile)
)

func redirectToAuth(c *gin.Context, oauthId string) {
	c.Abort()
	params := url.Values{
		"client_id":     {oauthId},
		"response_type": {"code"},
		"scope":         {"global"},
		// "state": "not used",
	}
	c.Redirect(302, "https://id.heroku.com/oauth/authorize?"+params.Encode())
}

func CheckAuth(cookieSecret string, oauthId string) gin.HandlerFunc {

	fernetKey, err := fernet.DecodeKey(cookieSecret)
	if err != nil {
		logger.Fatal("Cookie secret is not a valid Fernet key")
	}

	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(CookieName)
		if err != nil {
			logger.Print("Request received without cookie. Redirecting to auth")
			redirectToAuth(c, oauthId)
			return
		}
		cookieBytes, err := base64.RawURLEncoding.DecodeString(cookie.Value)
		if err != nil {
			logger.Print("Invalid cookie. Redirecting to auth")
			redirectToAuth(c, oauthId)
			return
		}
		accessToken := fernet.VerifyAndDecrypt(cookieBytes, -1, []*fernet.Key{fernetKey})
		if accessToken == nil {
			logger.Print("Cookie not valid or expired. Redirecting to auth")
			redirectToAuth(c, oauthId)
			return
		}
		c.Set("heroku",
			&heroku.Client{
				&heroku.Authorization{
					AccessToken: string(accessToken),
					TokenType:   "Bearer",
				}})
		c.Next()
	}
}

func HandleCallback(cookieSecret string, oauthSecret string) gin.HandlerFunc {

	fernetKey, err := fernet.DecodeKey(cookieSecret)
	if err != nil {
		// Fatal is ok because this function should always be called during server
		// initialization.
		logger.Fatal("Cookie secret is not a valid Fernet key")
	}

	return func(c *gin.Context) {
		code := c.Query("code")
		//state := c.Query("state")
		client, err := heroku.NewClientFromCode(oauthSecret, code)
		if err != nil {
			c.String(400, "OAuth failure: "+err.Error())
			return
		}
		encryptedBytes, err := fernet.EncryptAndSign([]byte(client.Authorization.AccessToken), fernetKey)
		if err != nil {
			c.String(500, "Internal error: "+err.Error())
			return
		}
		c.SetCookie(CookieName, base64.RawURLEncoding.EncodeToString(encryptedBytes), 3600, "", "", true, true)
		c.Redirect(302, "/manage/orgs")
	}
}

func HerokuClient(c *gin.Context) *heroku.Client {
	val, exists := c.Get("heroku")
	if !exists {
		logger.Print("WARNING! heroku client not found in context as expected. This error is not checked. Expect other errors!")
		return &heroku.Client{}
	}
	return val.(*heroku.Client)
}
