package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"leonlib/internal/auth"
	"leonlib/internal/captcha"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"

	"github.com/BurntSushi/toml"
)

type PageVariables struct {
	Year    string
	SiteKey string
}

type PageVariablesForAuthors struct {
	Year    string
	SiteKey string
	Authors []string
}

type PageResultsVariables struct {
	Year    string
	SiteKey string
	Results []BookInfo
}

type BookInfo struct {
	ID            int
	Title         string
	Author        string
	Description   string
	HasBeenRead   bool
	ImageName     string
	Image         []byte
	Base64Image   string
	AddedOn       string
	GoodreadsLink string
}

type Library struct {
	Book []BookInfo
}

func (bi BookInfo) String() string {
	return fmt.Sprintf(`%d) "%s" by "%s"`, bi.ID, bi.Title, bi.Author)
}

type BookSearchType int

const (
	Unknown BookSearchType = iota
	ByTitle
	ByAuthor
)

func (bt BookSearchType) String() string {
	switch bt {
	case ByTitle:
		return "ByTitle"
	case ByAuthor:
		return "ByAuthor"
	default:
		return "Unknown"
	}
}

func parseBookSearchType(input string) BookSearchType {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "bytitle":
		return ByTitle
	case "byauthor":
		return ByAuthor
	default:
		return Unknown
	}
}

// { "status" : "error" | "liked" | "not-liked" }
type LikeStatus struct {
	Status string
}

func Index(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	pageVariables := PageVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "index.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
	}
}

func BooksByAuthor(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	pageVariables := PageVariablesForAuthors{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
	}

	authors, err := getAllAuthors(db)
	if err != nil {
		log.Printf("Error getting authors: %v", err)
		redirectToErrorPage(w, r)
		return
	}

	log.Printf("debug:x authors=(%s)", authors)

	pageVariables.Authors = authors

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "books_by_author.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
	}
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(b)
}

//func Autocomplete(db *sql.DB, w http.ResponseWriter, r *http.Request) {
//	query := r.URL.Query().Get("q")
//
//	searchTypesStr := r.URL.Query().Get("searchType")
//	searchTypes := strings.Split(searchTypesStr, ",")
//
//	var suggestions []string
//
//	var queryStr string
//	var rows *sql.Rows
//	var err error
//
//	// Perform DB query based on queryParam("q")
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(map[string][]string{
//		"suggestions": suggestions,
//	})
//}

func BooksList(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	authorParam := r.URL.Query().Get("start_with")

	booksByAuthor, err := getBooksBySearchTypeCoincidence(db, authorParam, ByAuthor)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	type BookDetail struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Author      string `json:"author"`
		Description string `json:"description"`
		Base64Image string `json:"image"`
	}

	var results []BookDetail

	for _, book := range booksByAuthor {
		bookDetail := BookDetail{}
		bookDetail.ID = book.ID
		bookDetail.Title = book.Title
		bookDetail.Author = book.Author
		bookDetail.Description = book.Description
		if len(book.Image) > 0 {
			base64Image := base64.StdEncoding.EncodeToString(book.Image)
			bookDetail.Base64Image = "data:image/jpeg;base64," + base64Image
		}

		results = append(results, bookDetail)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func BooksCount(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	queryStr := `SELECT count(*) FROM books`
	rows, err := db.Query(queryStr)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var count int

	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"booksCount": count,
	})
}

func getAllAuthors(db *sql.DB) ([]string, error) {
	var err error

	allAuthorsRows, err := db.Query("SELECT DISTINCT author FROM books;")
	if err != nil {
		return []string{}, err
	}

	defer allAuthorsRows.Close()

	var authors []string
	for allAuthorsRows.Next() {
		var author string
		if err := allAuthorsRows.Scan(&author); err != nil {
			return []string{}, err
		}
		authors = append(authors, author)
	}

	return authors, nil
}

func getBookByID(db *sql.DB, id int) (BookInfo, error) {
	var err error
	var queryStr = `SELECT b.id, b.title, b.author, b.image, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.id=$1`

	bookRows, err := db.Query(queryStr, id)
	if err != nil {
		return BookInfo{}, err
	}

	var bookInfo BookInfo
	var bookID int
	var title string
	var author string
	var image []byte
	var description string
	var hasBeenRead bool
	var addedOn time.Time
	var goodreadsLink string
	if bookRows.Next() {
		if err := bookRows.Scan(&bookID, &title, &author, &image, &description, &hasBeenRead, &addedOn, &goodreadsLink); err != nil {
			return BookInfo{}, err
		}

		bookInfo.ID = bookID
		bookInfo.Title = title
		bookInfo.Author = author
		bookInfo.Image = image
		base64Image := ""
		if len(bookInfo.Image) > 0 {
			base64Image = base64.StdEncoding.EncodeToString(bookInfo.Image)
		}
		bookInfo.Base64Image = base64Image
		bookInfo.Description = description
		bookInfo.HasBeenRead = hasBeenRead
		bookInfo.AddedOn = addedOn.Format("2006-01-02")
	}

	return bookInfo, nil
}

func getBooksBySearchTypeCoincidence(db *sql.DB, titleSearchText string, bookSearchType BookSearchType) ([]BookInfo, error) {
	var err error
	var queryStr = `SELECT b.id, b.title, b.author, b.image, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.title ILIKE $1`

	if bookSearchType == ByAuthor {
		queryStr = `SELECT b.id, b.title, b.author, b.image, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.author ILIKE $1`
	}

	booksByTitleRows, err := db.Query(queryStr, "%"+titleSearchText+"%")
	if err != nil {
		return []BookInfo{}, err
	}

	defer booksByTitleRows.Close()

	var books []BookInfo
	var id int
	var title string
	var author string
	var image []byte
	var description string
	var hasBeenRead bool
	var addedOn time.Time
	var goodreadsLink string
	for booksByTitleRows.Next() {
		var bookInfo BookInfo
		if err := booksByTitleRows.Scan(&id, &title, &author, &image, &description, &hasBeenRead, &addedOn, &goodreadsLink); err != nil {
			return []BookInfo{}, err
		}

		bookInfo.ID = id
		bookInfo.Title = title
		bookInfo.Author = author
		bookInfo.Image = image
		base64Image := ""
		if len(bookInfo.Image) > 0 {
			base64Image = base64.StdEncoding.EncodeToString(bookInfo.Image)
		}
		bookInfo.Base64Image = base64Image
		bookInfo.Description = description
		bookInfo.HasBeenRead = hasBeenRead
		bookInfo.AddedOn = addedOn.Format("2006-01-02")
		books = append(books, bookInfo)
	}

	return books, nil
}

func redirectToErrorPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/error", http.StatusSeeOther)
}

func uniqueSearchTypes(searchTypes []string) []string {
	set := make(map[string]struct{})
	var result []string

	for _, item := range searchTypes {
		if _, exists := set[item]; !exists {
			set[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

/* Search for books */
func SearchBooks(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	bookQuery := r.URL.Query().Get("textSearch")
	searchTypesStr := r.URL.Query().Get("searchType")
	searchTypesParams := uniqueSearchTypes(strings.Split(searchTypesStr, ","))

	if len(searchTypesParams) == 0 || (len(searchTypesParams) == 1 && searchTypesParams[0] == "") {
		searchTypesParams = []string{"byTitle"}
	}

	fmt.Printf("debug:x textSearch=(%s), searchTypesParams=(%s)\n", bookQuery, searchTypesParams)

	var results []BookInfo
	var err error

	for _, searchTypeParam := range searchTypesParams {
		searchType := parseBookSearchType(searchTypeParam)
		switch searchType {
		case ByTitle:
			booksByTitle, err := getBooksBySearchTypeCoincidence(db, bookQuery, ByTitle)
			if err != nil {
				log.Printf("Error parsing template: %v", err)
				redirectToErrorPage(w, r)
				return
			}
			log.Printf("Got=(%s)", booksByTitle)
			results = append(results, booksByTitle...)

		case ByAuthor:
			booksByAuthor, err := getBooksBySearchTypeCoincidence(db, bookQuery, ByAuthor)
			if err != nil {
				redirectToErrorPage(w, r)
				return
			}
			results = append(results, booksByAuthor...)

		case Unknown:
			log.Printf("Tipo de búsqueda en libros desconocido.")
			redirectToErrorPage(w, r)

			return
		}
	}

	now := time.Now()
	pageVariables := PageResultsVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
		Results: results,
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "search_books.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		redirectToErrorPage(w, r)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Descripción del error: %v", err)
		return
	}
}

func ErrorPage(w http.ResponseWriter, _ *http.Request) {
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "error5xx.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func Ingresar(w http.ResponseWriter, r *http.Request) {
	oauthState := generateRandomString(32)

	session, _ := auth.SessionStore.Get(r, "user-session")
	session.Values["oauth_state"] = oauthState
	session.Save(r, w)

	url := auth.GoogleOauthConfig.AuthCodeURL(oauthState)

	http.Redirect(w, r, url, http.StatusSeeOther)
}

func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := auth.GoogleOauthConfig.AuthCodeURL("random_state")
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func GoogleCallback(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	token, err := auth.GoogleOauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "No se pudo obtener el token de Google", http.StatusInternalServerError)
		return
	}

	oauth2Service, err := oauth2.NewService(r.Context(), option.WithTokenSource(auth.GoogleOauthConfig.TokenSource(r.Context(), token)))
	if err != nil {
		http.Error(w, "No se pudo crear el servicio OAuth2", http.StatusInternalServerError)
		return
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		http.Error(w, "No se pudo obtener la información del usuario", http.StatusInternalServerError)
		return
	}

	fmt.Println("ID:", userInfo.Id)
	fmt.Println("Email:", userInfo.Email)
	fmt.Println("Name:", userInfo.Name)

	_, err = db.Exec(`-- noinspection SqlResolveForFile
	
			INSERT INTO users(user_id, email, name, oauth_identifier) 
			VALUES($1, $2, $3, $4)
			ON CONFLICT(user_id) DO UPDATE
			SET email = $2, name = $3`, userInfo.Id, userInfo.Email, userInfo.Name, "Google")

	if err != nil {
		http.Error(w, "Error al guardar el usuario en la base de datos", http.StatusInternalServerError)
		return
	}

	session, _ := auth.SessionStore.Get(r, "user-session")
	session.Values["user_id"] = userInfo.Id

	fmt.Println(session)

	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func getCurrentUserID(r *http.Request) (string, error) {
	session, err := auth.SessionStore.Get(r, "user-session")
	if err != nil {
		return "", err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		return "", errors.New("user_id not found in session")
	}

	fmt.Println("--------")
	fmt.Println(session)
	fmt.Println(userID)
	fmt.Println("----- end")

	return userID, nil
}

func writeErrorGeneralStatus(w http.ResponseWriter, err error) {
	log.Printf("Error parsing template: %v", err)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
	})
}

func LikeBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	/*
		userID, err := getCurrentUserID(r)
		if err != nil {
			http.Error(w, "2) Error al obtener información de la sesión", http.StatusInternalServerError)
			return
		}*/
	var err error

	r.ParseForm()
	bookID := r.PostFormValue("book_id")

	_, err = db.Exec("INSERT INTO book_likes(book_id) VALUES($1)", bookID)

	if err != nil {
		http.Error(w, "Error al dar like en la base de datos", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Liked successfully"))
}

func LikesCount(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		http.Error(w, "book_id is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(bookID)
	if err != nil {
		http.Error(w, "Invalid book_id", http.StatusBadRequest)
		return
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM book_likes WHERE book_id = $1", id).Scan(&count)
	if err != nil {
		http.Error(w, "Error querying the database", http.StatusInternalServerError)
		return
	}

	resp := map[string]int{
		"count": count,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func CreateDBFromFile(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	libraryDir := "library"
	libraryDirPath := filepath.Join(libraryDir, "books_db.toml")

	var library Library

	if _, err := toml.DecodeFile(libraryDirPath, &library); err != nil {
		writeErrorGeneralStatus(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	for _, book := range library.Book {
		log.Printf("Reading: (%s)", book)

		imgBytes, err := os.ReadFile(filepath.Join("images", book.ImageName))
		if err != nil {
			writeErrorGeneralStatus(w, err)
			return
		}

		stmt, err := db.Prepare("INSERT INTO books(title, author, image, description, read, added_on, goodreads_link) VALUES($1, $2, $3, $4, $5, $6, $7)")
		if err != nil {
			writeErrorGeneralStatus(w, err)
			return
		}

		_, err = stmt.Exec(book.Title, book.Author, imgBytes, book.Description, book.HasBeenRead, book.AddedOn, book.GoodreadsLink)
		if err != nil {
			writeErrorGeneralStatus(w, err)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}

func InfoBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	idQueryParam := r.URL.Query().Get("id")

	id, err := strconv.Atoi(idQueryParam)
	if err != nil {
		redirectToErrorPage(w, r)
		return
	}

	bookByID, err := getBookByID(db, id)
	if err != nil {
		redirectToErrorPage(w, r)
		return
	}

	now := time.Now()

	fmt.Printf("debug:x SiteKey=(%s)\n", captcha.SiteKey)

	pageVariables := PageResultsVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
		Results: []BookInfo{bookByID},
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "book_info.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		redirectToErrorPage(w, r)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Descripción del error: %v", err)
		return
	}
}
