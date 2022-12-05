package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	newServer()
}

func newServer() {
	gin.SetMode(gin.ReleaseMode)
	s := gin.New()
	s.LoadHTMLFiles("./internal/server/html.html")
	s.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "html.html", map[string]interface{}{
			"code": c.Query("code"),
		})
	})

	go s.Run(":8080")
}
