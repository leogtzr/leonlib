package auth

import (
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	SessionStore *sessions.CookieStore
	Auth0Config  *oauth2.Config
)
