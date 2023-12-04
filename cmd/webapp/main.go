package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"leonlib/internal/auth"
	"leonlib/internal/captcha"
	"leonlib/internal/router"
	"log"
	"net/http"
	"os"
)

var (
	dbMode      = os.Getenv("DB_MODE")
	ctx         = context.Background()
	mainAppUser = os.Getenv("LEONLIB_MAINAPP_USER")
	DB          *sql.DB
)

func init() {
	if dbMode == "" {
		log.Fatal("error: DB_MODE not defined")
	}
	if mainAppUser == "" {
		log.Fatal("error: LEONLIB_MAINAPP_USER not defined")
	}
	captcha.SiteKey = os.Getenv("LEONLIB_CAPTCHA_SITE_KEY")
	captcha.SecretKey = os.Getenv("LEONLIB_CAPTCHA_SECRET_KEY")
	if captcha.SiteKey == "" {
		log.Fatal("error: LEONLIB_CAPTCHA_SITE_KEY not defined")
	}
	if captcha.SecretKey == "" {
		log.Fatal("error: LEONLIB_CAPTCHA_SECRET_KEY not defined")
	}

	auth.Config = &oauth2.Config{
		ClientID:     os.Getenv("AUTH0_CLIENT_ID"),
		ClientSecret: os.Getenv("AUTH0_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("AUTH0_CALLBACK_URL"),
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://" + os.Getenv("AUTH0_DOMAIN") + "/authorize",
			TokenURL: "https://" + os.Getenv("AUTH0_DOMAIN") + "/oauth/token",
		},
	}

	auth.SessionStore = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
}

type DBConnectionInfo struct {
	driverName    string
	connectionURL string
}

func getConnectionInfo() (DBConnectionInfo, error) {
	switch dbMode {
	case "inmemory":
		return DBConnectionInfo{
			driverName:    "sqlite3",
			connectionURL: "/var/lib/appdata/leonlib.db",
		}, nil
	}

	return DBConnectionInfo{}, fmt.Errorf("wrong DB mode")
}

func initDB() (*sql.DB, error) {
	switch dbMode {
	case "inmemory":
		return sql.Open("sqlite3", "/var/lib/appdata/leonlib.db")
	}

	return nil, fmt.Errorf("wrong DB mode")
}

func main() {
	DB, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = DB.Close()
	}()

	err = DB.Ping()
	if err != nil {
		panic(err)
	}

	sqlCommands := []string{
		`CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			author TEXT NOT NULL,
			description TEXT,
			read BOOLEAN DEFAULT FALSE,
			added_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			goodreads_link TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS book_images (
			image_id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL REFERENCES books(id),
			image BLOB NOT NULL,
			added_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			user_id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			oauth_identifier TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS book_likes (
			like_id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER REFERENCES books(id),
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_id TEXT REFERENCES users(user_id),
			UNIQUE(book_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_books_title ON books (title)`,
		`CREATE INDEX IF NOT EXISTS idx_books_author ON books (author)`,
		`CREATE INDEX IF NOT EXISTS idx_books_added_on ON books (added_on)`,
		`CREATE INDEX IF NOT EXISTS idx_book_images_book_id ON book_images (book_id)`,
	}

	for _, sqlCommand := range sqlCommands {
		_, err := DB.Exec(sqlCommand)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("SQL command: (%.35s...) executed correctly", sqlCommand)
	}

	r := router.NewRouter(DB)

	fs := http.FileServer(http.Dir("assets/"))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8180"
	}

	log.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
