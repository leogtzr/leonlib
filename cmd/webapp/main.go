package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"leonlib/internal/auth"
	"leonlib/internal/captcha"
	"leonlib/internal/router"
	"log"
	"net/http"
	"os"
)

var (
	dbHost      = os.Getenv("PGHOST")
	dbUser      = os.Getenv("PGUSER")
	dbPassword  = os.Getenv("POSTGRES_PASSWORD")
	dbName      = os.Getenv("PGDATABASE")
	dbPort      = os.Getenv("PGPORT")
	DB          *sql.DB
	ctx         = context.Background()
	databaseURL = os.Getenv("DATABASE_URL")
)

func init() {
	captcha.SiteKey = os.Getenv("LEONLIB_CAPTCHA_SITE_KEY")
	captcha.SecretKey = os.Getenv("LEONLIB_CAPTCHA_SECRET_KEY")
	if captcha.SiteKey == "" {
		log.Fatal("error: LEONLIB_CAPTCHA_SITE_KEY not defined")
	}
	if captcha.SecretKey == "" {
		log.Fatal("error: LEONLIB_CAPTCHA_SECRET_KEY not defined")
	}

	auth.GoogleOauthConfig = &oauth2.Config{
		RedirectURL:  os.Getenv("LEONLIB_GOOGLE_OATH_CALLBACK"),
		ClientID:     os.Getenv("LEONLIB_GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("LEONLIB_GOOGLE_OAUTH_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	auth.SessionStore = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
}

func main() {
	var psqlInfo string

	if databaseURL == "" {
		psqlInfo = "host=" + dbHost + " port=" + dbPort + " user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"
	} else {
		psqlInfo = databaseURL
	}

	fmt.Printf("debug:x connection=(%s)\n", psqlInfo)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	err = DB.Ping()
	if err != nil {
		panic(err)
	}

	defer DB.Close()

	router := router.NewRouter(DB)

	fs := http.FileServer(http.Dir("assets/"))
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8180"
	}

	log.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
