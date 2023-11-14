package router

import (
	"database/sql"
	"leonlib/internal/handler"
	"net/http"

	"github.com/gorilla/mux"
)

type Router struct {
	Name        string
	Method      string
	Path        string
	HandlerFunc http.HandlerFunc
}

type Routes []Router

var routes Routes

func initRoutes(db *sql.DB) {
	routes = Routes{
		Router{
			"",
			"GET",
			"/adm/initdb",
			func(w http.ResponseWriter, r *http.Request) {
				handler.CreateDBFromFile(db, w, r)
			},
		},
		Router{
			"LikesCount",
			"GET",
			"/api/likes_count",
			func(w http.ResponseWriter, r *http.Request) {
				handler.LikesCount(db, w, r)
			},
		},
		Router{
			"Like Book",
			"POST",
			"/api/like",
			func(w http.ResponseWriter, r *http.Request) {
				handler.LikeBook(db, w, r)
			},
		},
		Router{
			"GoogleAuth",
			"GET",
			"/auth/google/login",
			handler.GoogleLogin,
		},
		Router{
			"GoogleCallback",
			"GET",
			"/auth/callback",
			func(w http.ResponseWriter, r *http.Request) {
				handler.GoogleCallback(db, w, r)
			},
		},
		Router{
			"Books by author",
			"GET",
			"/books_by_author",
			func(w http.ResponseWriter, r *http.Request) {
				handler.BooksByAuthor(db, w, r)
			},
		},
		Router{
			"ErrorPage",
			"GET",
			"/error",
			handler.ErrorPage,
		},
		Router{
			"Index",
			"GET",
			"/",
			handler.Index,
		},
		Router{
			"Search for books",
			"GET",
			"/search_books",
			func(w http.ResponseWriter, r *http.Request) {
				handler.SearchBooks(db, w, r)
			},
		},
		Router{
			"Book Info",
			"GET",
			"/book_info",
			func(w http.ResponseWriter, r *http.Request) {
				handler.InfoBook(db, w, r)
			},
		},
		Router{
			"Ingresar",
			"GET",
			"/ingresar",
			handler.Ingresar,
		},
		//Router{
		//	"Autocomplete",
		//	"GET",
		//	"/api/autocomplete",
		//	func(w http.ResponseWriter, r *http.Request) {
		//		handler.Autocomplete(db, w, r)
		//	},
		//},
		Router{
			"Books Count",
			"GET",
			"/api/booksCount",
			func(w http.ResponseWriter, r *http.Request) {
				handler.BooksCount(db, w, r)
			},
		},
		Router{
			"Books List",
			"GET",
			"/api/books",
			func(w http.ResponseWriter, r *http.Request) {
				handler.BooksList(db, w, r)
			},
		},
	}
}

func NewRouter(db *sql.DB) *mux.Router {
	initRoutes(db)
	router := mux.NewRouter().StrictSlash(true)

	//rateLimiter := middleware.NewRateLimiterMiddleware(ratelimit.RedisClient, 1, 5)

	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Path).
			Name(route.Name).
			Handler(http.HandlerFunc(route.HandlerFunc))
	}

	fs := http.FileServer(http.Dir("assets/"))
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

	return router
}
