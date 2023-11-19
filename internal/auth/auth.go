package auth

import (
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	SessionStore *sessions.CookieStore
	Config       *oauth2.Config
)
