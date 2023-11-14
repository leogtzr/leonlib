package auth

import (
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	GoogleOauthConfig *oauth2.Config
	SessionStore      *sessions.CookieStore
)
