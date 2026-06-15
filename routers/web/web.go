package web

import (
	"net/http"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/web"
	"github.com/gin-gonic/gin"
)

// Router for frontend web
type Router struct {
	Config          *config.Config
	LoginMiddleware core.LoginMiddleware
	SCMService      core.SCMService
	Session         core.Session
}

// RegisterRoutes for Gin
func (r *Router) RegisterRoutes(e *gin.Engine) {
	{
		scm := r.Config.Provider()
		e.Any("/login",
			MiddlewareBindUser(r.Session),
			MiddlewareLogin(scm, r.LoginMiddleware),
			HandleLogin(r.Config, scm, r.SCMService, r.Session),
		)
	}
	e.Any("/logoff", HandleLogout(r.Config, r.Session))
	h := gin.WrapH(http.FileServer(web.New()))
	e.GET("/favicon.ico", h)
	e.GET("/logo.png", h)
	e.GET("/assets/*filepath", h)
	e.NoRoute(HandleIndex(r.Config))
}
