package server

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed html.html
var htmlTmpl string

func init() {
	newServer()
}

func newServer() {
	t := template.New("")
	t, err := t.New("html.html").Parse(htmlTmpl)
	if err != nil {
		log.Fatalln("failed to parse html.tmpl")
	}

	gin.SetMode(gin.ReleaseMode)
	s := gin.New()
	s.SetHTMLTemplate(t)
	s.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "html.html", map[string]interface{}{
			"code": c.Query("code"),
		})
	})

	go s.Run(":8080")
}
