package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"leonlib/internal/auth"
	"leonlib/internal/captcha"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/BurntSushi/toml"
)

const (
	Unknown BookSearchType = iota
	ByTitle
	ByAuthor
)

type BookInfo struct {
	ID            int
	Title         string
	Author        string
	Description   string
	HasBeenRead   bool
	ImageNames    []string
	Image         []byte
	Base64Images  []BookImageInfo
	AddedOn       string
	GoodreadsLink string
}

type BookImageInfo struct {
	ImageID int
	BookID  int
	Image   string
}

type BookSearchType int

type RequestData struct {
	BookID string `json:"book_id"`
}

type PageVariables struct {
	Year     string
	SiteKey  string
	LoggedIn bool
}

type PageVariablesForAuthors struct {
	Year     string
	SiteKey  string
	Authors  []string
	LoggedIn bool
}

type PageResultsVariables struct {
	Year     string
	SiteKey  string
	Results  []BookInfo
	LoggedIn bool
}

type UserInfo struct {
	Sub      string `json:"sub"`            // Identificador único del usuario
	Name     string `json:"name"`           // Nombre completo del usuario
	Nickname string `json:"nickname"`       // Apodo del usuario
	Picture  string `json:"picture"`        // URL de la imagen de perfil del usuario
	Email    string `json:"email"`          // Correo electrónico del usuario
	Verified bool   `json:"email_verified"` // Si el correo electrónico está verificado
}

type Library struct {
	Book []BookInfo
}

// LikeStatus { "status" : "error" | "liked" | "not-liked" }
type LikeStatus struct {
	Status string
}

func (bi BookInfo) String() string {
	return fmt.Sprintf(`%d) "%s" by "%s"`, bi.ID, bi.Title, bi.Author)
}

func (ui UserInfo) String() string {
	return fmt.Sprintf("Name=(%s), email=(%s), nickname=(%s), verified=(%t), sub=(%s)", ui.Name, ui.Email, ui.Nickname, ui.Verified, ui.Sub)
}

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

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(b)
}

func getDatabaseEmailFromSessionID(db *sql.DB, userID string) (string, error) {
	queryStr := "SELECT u.email FROM users u WHERE u.user_id=$1"

	rows, err := db.Query(queryStr, userID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var email string

	if rows.Next() {
		if err := rows.Scan(&email); err != nil {
			return "", err
		}
	}

	return email, nil
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

func getUserInfoFromAuth0(accessToken string) (*UserInfo, error) {
	userInfoEndpoint := fmt.Sprintf("https://%s/userinfo", os.Getenv("AUTH0_DOMAIN"))

	req, err := http.NewRequest("GET", userInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando la solicitud: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al realizar la solicitud: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer la respuesta: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error en la respuesta de Auth0: %s", body)
	}

	var userInfo UserInfo
	err = json.Unmarshal(body, &userInfo)
	if err != nil {
		return nil, fmt.Errorf("error al decodificar la respuesta JSON: %v", err)
	}

	return &userInfo, nil
}

func redirectToErrorPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/error", http.StatusSeeOther)
}

func redirectToErrorPageWithMessageAndStatusCode(w http.ResponseWriter, errorMessage string, httpStatusCode int) {
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "error5xx.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type ErrorVariables struct {
		Year         string
		ErrorMessage string
	}

	now := time.Now()

	pageVariables := ErrorVariables{
		Year:         now.Format("2006"),
		ErrorMessage: errorMessage,
	}

	w.WriteHeader(httpStatusCode)

	err = t.Execute(w, pageVariables)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
}

func writeErrorGeneralStatus(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
	})
}

func writeErrorLikeStatus(w http.ResponseWriter, err error) {
	log.Printf("Error parsing template: %v", err)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
	})
}

func writeUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{"status": "unauthenticated"})
}

func getCurrentUserID(r *http.Request) (string, error) {
	session, err := auth.SessionStore.Get(r, "user-session")
	if err != nil {
		return "", err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		return "", errors.New("0) user_id not found in session")
	}

	fmt.Println("--------")
	fmt.Println(session)
	fmt.Println(userID)
	fmt.Println("----- end")

	return userID, nil
}

func getAllAuthors(db *sql.DB) ([]string, error) {
	var err error

	allAuthorsRows, err := db.Query("SELECT DISTINCT author FROM books ORDER BY author")
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

func getImagesByBookID(db *sql.DB, bookID int) ([]BookImageInfo, error) {
	bookImagesRows, err := db.Query(`SELECT i.image_id, i.book_id, i.image FROM book_images i WHERE i.book_id=$1`, bookID)
	if err != nil {
		return []BookImageInfo{}, err
	}

	defer func() {
		_ = bookImagesRows.Close()
	}()

	var images []BookImageInfo

	for bookImagesRows.Next() {
		var imageID int
		var bookID int
		var base64Image []byte
		if err = bookImagesRows.Scan(&imageID, &bookID, &base64Image); err != nil {
			return []BookImageInfo{}, err
		}

		if len(base64Image) > 0 {
			encodedImage := base64.StdEncoding.EncodeToString(base64Image)
			bookImageInfo := BookImageInfo{
				ImageID: imageID,
				BookID:  bookID,
				Image:   encodedImage,
			}
			images = append(images, bookImageInfo)
		}
	}

	return images, nil
}

func getBookByID(db *sql.DB, id int) (BookInfo, error) {
	var err error
	var queryStr = `SELECT b.id, b.title, b.author, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.id=$1`

	bookRows, err := db.Query(queryStr, id)
	if err != nil {
		return BookInfo{}, err
	}

	defer func() {
		bookRows.Close()
	}()

	var bookInfo BookInfo
	var bookID int
	var title string
	var author string
	var description string
	var hasBeenRead bool
	var addedOn time.Time
	var goodreadsLink sql.NullString
	if bookRows.Next() {
		if err := bookRows.Scan(&bookID, &title, &author, &description, &hasBeenRead, &addedOn, &goodreadsLink); err != nil {
			return BookInfo{}, err
		}

		bookInfo.ID = bookID
		bookInfo.Title = title
		bookInfo.Author = author
		bookInfo.Description = description
		bookInfo.HasBeenRead = hasBeenRead
		bookInfo.AddedOn = addedOn.Format("2006-01-02")
		if goodreadsLink.Valid {
			bookInfo.GoodreadsLink = goodreadsLink.String
		} else {
			bookInfo.GoodreadsLink = "" // o cualquier valor predeterminado
		}
	}

	bookImages, err := getImagesByBookID(db, id)
	if err != nil {
		return BookInfo{}, err
	}

	bookInfo.Base64Images = bookImages

	return bookInfo, nil
}

func getBooksBySearchTypeCoincidence(db *sql.DB, titleSearchText string, bookSearchType BookSearchType) ([]BookInfo, error) {
	var err error
	var queryStr = `SELECT b.id, b.title, b.author, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.title ILIKE $1 ORDER BY b.title`

	if bookSearchType == ByAuthor {
		queryStr = `SELECT b.id, b.title, b.author, b.description, b.read, b.added_on, b.goodreads_link FROM books b WHERE b.author ILIKE $1 ORDER BY b.title`
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
	var description string
	var hasBeenRead bool
	var addedOn time.Time
	var goodreadsLink string
	for booksByTitleRows.Next() {
		var bookInfo BookInfo
		if err := booksByTitleRows.Scan(&id, &title, &author, &description, &hasBeenRead, &addedOn, &goodreadsLink); err != nil {
			return []BookInfo{}, err
		}

		bookInfo.ID = id
		bookInfo.Title = title
		bookInfo.Author = author
		bookImages, err := getImagesByBookID(db, id)
		if err != nil {
			return []BookInfo{}, err
		}

		bookInfo.Base64Images = bookImages
		bookInfo.Description = description
		bookInfo.HasBeenRead = hasBeenRead
		bookInfo.AddedOn = addedOn.Format("2006-01-02")
		books = append(books, bookInfo)
	}

	return books, nil
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

func IndexPage(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	pageVariables := PageVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
	}

	_, err := getCurrentUserID(r)
	if err != nil {
		log.Printf("User is not logged in: %v", err)
		pageVariables.LoggedIn = false
	} else {
		log.Println("User is logged in")
		pageVariables.LoggedIn = true
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "index.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)

		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

func BooksByAuthorPage(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	pageVariables := PageVariablesForAuthors{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
	}

	authors, err := getAllAuthors(db)
	if err != nil {
		log.Printf("Error getting authors: %v", err)
		redirectToErrorPageWithMessageAndStatusCode(w, "error getting information from the database", http.StatusInternalServerError)
		return
	}

	_, err = getCurrentUserID(r)
	if err != nil {
		log.Printf("(BooksByAuthorPage) User is not logged in: %v", err)
		pageVariables.LoggedIn = false
	} else {
		log.Println("User is logged in")
		pageVariables.LoggedIn = true
	}

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
		ID           int             `json:"id"`
		Title        string          `json:"title"`
		Author       string          `json:"author"`
		Description  string          `json:"description"`
		Base64Images []BookImageInfo `json:"images"`
	}

	var results []BookDetail

	for _, book := range booksByAuthor {
		bookDetail := BookDetail{}
		bookDetail.ID = book.ID
		bookDetail.Title = book.Title
		bookDetail.Author = book.Author
		bookDetail.Description = book.Description
		bookDetail.Base64Images = book.Base64Images

		results = append(results, bookDetail)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func BooksCount(db *sql.DB, w http.ResponseWriter) {
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

func SearchBooksPage(db *sql.DB, w http.ResponseWriter, r *http.Request) {
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
				redirectToErrorPageWithMessageAndStatusCode(w, "Error getting information from the database", http.StatusInternalServerError)

				return
			}
			results = append(results, booksByTitle...)

		case ByAuthor:
			booksByAuthor, err := getBooksBySearchTypeCoincidence(db, bookQuery, ByAuthor)
			if err != nil {
				log.Printf("error getting info from the database: %v", err)
				redirectToErrorPageWithMessageAndStatusCode(w, "error getting info from the database", http.StatusInternalServerError)
				return
			}
			results = append(results, booksByAuthor...)

		case Unknown:
			log.Printf("Tipo de búsqueda en libros desconocido.")
			redirectToErrorPageWithMessageAndStatusCode(w, "Wrong search", http.StatusInternalServerError)

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
		templateDir = "internal/template"
	}
	templatePath := filepath.Join(templateDir, "search_books.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("template error: %v", err)
		redirectToErrorPageWithMessageAndStatusCode(w, "template error", http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error: %v", err)
		return
	}
}

func ErrorPage(w http.ResponseWriter, _ *http.Request) {
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template"
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

func IngresarPage(w http.ResponseWriter, r *http.Request) {
	oauthState := generateRandomString(32)

	session, _ := auth.SessionStore.Get(r, "user-session")
	session.Values["oauth_state"] = oauthState
	session.Save(r, w)

	//url := auth.GoogleOauthConfig.AuthCodeURL(oauthState)
	url := auth.Config.AuthCodeURL(oauthState)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func Auth0Callback(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")

	token, err := auth.Config.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, "Cannot get Auth0 token", http.StatusInternalServerError)
		return
	}

	userInfo, err := getUserInfoFromAuth0(token.AccessToken)
	if err != nil {
		log.Printf("error: cannot get user info from Auth0: %v", err)
		http.Error(w, "cannot get user info from Auth0", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`
			INSERT INTO users(user_id, email, name, oauth_identifier) 
			VALUES($1, $2, $3, $4)
			ON CONFLICT(user_id) DO UPDATE
			SET email = $2, name = $3`, userInfo.Sub, userInfo.Email, userInfo.Name, "Google")

	if err != nil {
		http.Error(w, "Error al guardar el usuario en la base de datos", http.StatusInternalServerError)
		return
	}

	session, _ := auth.SessionStore.Get(r, "user-session")
	session.Values["user_id"] = userInfo.Sub
	session.Save(r, w)

	now := time.Now()

	pageVariables := PageVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
	}

	_, err = getCurrentUserID(r)
	if err != nil {
		pageVariables.LoggedIn = false
	} else {
		pageVariables.LoggedIn = true
	}

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // valor predeterminado para desarrollo local
	}
	templatePath := filepath.Join(templateDir, "index.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error al analizar la plantilla: %v", err)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error al ejecutar la plantilla: %v", err)
	}
}

func CheckLikeStatus(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	userID, err := getCurrentUserID(r)
	if err != nil {
		writeUnauthenticated(w)

		return
	}

	vars := mux.Vars(r)
	wordID := vars["word_id"]

	queryStr := "SELECT EXISTS(SELECT 1 FROM book_likes WHERE book_id=$1 AND user_id=$2)"

	rows, err := db.Query(queryStr, wordID, userID)
	if err != nil {
		writeErrorLikeStatus(w, err)
		return
	}
	defer rows.Close()

	var exists bool

	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			writeErrorLikeStatus(w, err)
			return
		}
	}

	if err != nil {
		writeErrorLikeStatus(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if exists {
		json.NewEncoder(w).Encode(map[string]string{"status": "liked"})
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "not-liked"})
	}
}

func LikeBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	userID, err := getCurrentUserID(r)
	if err != nil {
		http.Error(w, "2) Error al obtener información de la sesión", http.StatusInternalServerError)
		return
	}

	err = r.ParseForm()
	if err != nil {
		w.Write([]byte(fmt.Sprintf("error like book: %v", err.Error())))
	}
	bookID := r.PostFormValue("book_id")

	_, err = db.Exec("INSERT INTO book_likes(book_id, user_id) VALUES($1, $2) ON CONFLICT(book_id, user_id) DO NOTHING", bookID, userID)

	if err != nil {
		http.Error(w, "Error al dar like en la base de datos", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Liked successfully"))
}

func AddBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(2 << 20) // Por ejemplo, 10 MB
	if err != nil {
		log.Printf("1) error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	description := r.FormValue("description")
	read := r.FormValue("read") == "on"
	goodreadsLink := r.FormValue("goodreadsLink")

	var imageData []byte
	file, _, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		imageData, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if !errors.Is(err, http.ErrMissingFile) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO books (title, author, image, description, read, goodreads_link) VALUES ($1, $2, $3, $4, $5, $6)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(title, author, imageData, description, read, goodreadsLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Libro agregado con éxito"))
}

func UnlikeBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	userID, err := getCurrentUserID(r)
	if err != nil {
		http.Error(w, "Error al obtener información de la sesión", http.StatusInternalServerError)
		return
	}

	var requestData RequestData

	err = json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Error al decodificar el cuerpo de la solicitud", http.StatusInternalServerError)
		return
	}

	bookID := requestData.BookID

	fmt.Printf("debug:x trying to unlike book_id=(%s), user_id=(%s)\n", bookID, userID)

	_, err = db.Exec("DELETE FROM book_likes WHERE book_id=$1 AND user_id=$2", bookID, userID)
	if err != nil {
		http.Error(w, "Error al quitar el like en la base de datos", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Unliked successfully"))
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

func CreateDBFromFile(db *sql.DB, w http.ResponseWriter) {
	libraryDir := "library"
	libraryDirPath := filepath.Join(libraryDir, "books_db.toml")

	var library Library

	if _, err := toml.DecodeFile(libraryDirPath, &library); err != nil {
		writeErrorGeneralStatus(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	startTime := time.Now()

	for _, book := range library.Book {
		log.Printf("Reading: (%s)", book)

		var bookID int
		stmt, err := db.Prepare("INSERT INTO books(title, author, description, read, added_on, goodreads_link) VALUES($1, $2, $3, $4, $5, $6) RETURNING id")
		if err != nil {
			writeErrorGeneralStatus(w, err)
			return
		}

		err = stmt.QueryRow(book.Title, book.Author, book.Description, book.HasBeenRead, book.AddedOn, book.GoodreadsLink).Scan(&bookID)
		if err != nil {
			writeErrorGeneralStatus(w, err)
			return
		}

		// Process images:
		for _, imageName := range book.ImageNames {
			imgBytes, err := os.ReadFile(filepath.Join("images", imageName))
			if err != nil {
				writeErrorGeneralStatus(w, err)
				return
			}

			imgStmt, err := db.Prepare("INSERT INTO book_images(book_id, image) VALUES($1, $2)")
			if err != nil {
				writeErrorGeneralStatus(w, err)
				return
			}

			_, err = imgStmt.Exec(bookID, imgBytes)
			if err != nil {
				writeErrorGeneralStatus(w, err)
				return
			}
		}
	}

	elapsedTime := time.Since(startTime)

	log.Printf("Books loaded in: %s\n", elapsedTime)

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
		log.Printf("error: getting information from the database")
		redirectToErrorPageWithMessageAndStatusCode(w, "error getting information from the database", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	pageVariables := PageResultsVariables{
		Year:    now.Format("2006"),
		SiteKey: captcha.SiteKey,
		Results: []BookInfo{bookByID},
	}

	_, err = getCurrentUserID(r)
	if err != nil {
		pageVariables.LoggedIn = false
	} else {
		pageVariables.LoggedIn = true
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
		log.Printf("template error: %v", err)
		return
	}
}

func ModifyBook(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// TODO: check auth here
	err := r.ParseMultipartForm(2 << 20)
	if err != nil {
		writeErrorGeneralStatus(w, err)

		return
	}

	bookIDParam := r.FormValue("book_id")
	title := r.FormValue("title")
	author := r.FormValue("author")
	description := r.FormValue("description")
	read := r.FormValue("read") == "on"
	goodreadsLink := r.FormValue("goodreadsLink")

	fmt.Printf("debug:x bookID=(%s), title=(%s), author=(%s), description=(%s), read=(%t), goodreadsLink=(%s)\n",
		bookIDParam, title, author, description, read, goodreadsLink)

	id, err := strconv.Atoi(bookIDParam)
	if err != nil {
		writeErrorGeneralStatus(w, err)

		return
	}

	file, _, err := r.FormFile("image")
	err = addImageToBook(db, id, r, file)
	if err != nil {
		writeErrorGeneralStatus(w, err)

		return
	}

	bookUpdate, err := db.Prepare(`
		UPDATE books SET 
			title = $1,
			author = $2,
			description = $3,
			read = $4,
			goodreads_link = $5
		WHERE id = $6
	`)
	if err != nil {
		writeErrorGeneralStatus(w, err)

		return
	}
	defer func() {
		_ = bookUpdate.Close()
	}()

	_, err = bookUpdate.Exec(title, author, description, read, goodreadsLink, id)
	if err != nil {
		writeErrorGeneralStatus(w, err)

		return
	}

	w.Write([]byte("Libro modificado con exito"))
}

func addImageToBook(db *sql.DB, id int, r *http.Request, file multipart.File) error {
	var imageData []byte
	file, _, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		imageData, err = io.ReadAll(file)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, http.ErrMissingFile) {
		return err
	}

	if len(imageData) == 0 {
		return nil
	}

	imgStmt, err := db.Prepare("INSERT INTO book_images(book_id, image) VALUES($1, $2)")
	if err != nil {
		return err
	}

	_, err = imgStmt.Exec(id, imageData)
	if err != nil {
		return err
	}

	return nil
}

func ModifyBookPage(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	idQueryParam := r.URL.Query().Get("book_id")

	id, err := strconv.Atoi(idQueryParam)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, "wrong ID", http.StatusInternalServerError)
		return
	}

	bookByID, err := getBookByID(db, id)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, "error getting information from the database", http.StatusInternalServerError)
		return
	}

	now := time.Now()

	type BookToModifyVariables struct {
		Year          string
		SiteKey       string
		Book          BookInfo
		LoggedIn      bool
		GoodreadsLink template.URL
	}

	pageVariables := BookToModifyVariables{
		Year:          now.Format("2006"),
		SiteKey:       captcha.SiteKey,
		Book:          bookByID,
		GoodreadsLink: template.URL(bookByID.GoodreadsLink),
	}

	//_, err = getCurrentUserID(r)
	//if err != nil {
	//	redirectToErrorPageWithMessageAndStatusCode(w, "Error al obtener información de la sesión", http.StatusInternalServerError)
	//
	//	return
	//}
	pageVariables.LoggedIn = true

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "modify.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

func AboutPage(w http.ResponseWriter, r *http.Request) {
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "about.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()

	pageVariables := PageVariables{
		Year:     now.Format("2006"),
		SiteKey:  captcha.SiteKey,
		LoggedIn: false,
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ContactPage(w http.ResponseWriter, _ *http.Request) {
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "contact.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()

	pageVariables := PageVariables{
		Year:     now.Format("2006"),
		SiteKey:  captcha.SiteKey,
		LoggedIn: false,
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		redirectToErrorPageWithMessageAndStatusCode(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func AddBookPage(w http.ResponseWriter, r *http.Request) {
	/*
		userID, err := getCurrentUserID(r)
		if err != nil {
			redirectToErrorPageWithMessageAndStatusCode(w, "Error al obtener información de la sesión", http.StatusInternalServerError)

			return
		}

		email, err := getDatabaseEmailFromSessionID(db, userID)

		if err != nil {
			redirectToErrorPageWithMessageAndStatusCode(w, "Only admins can access to this page", http.StatusForbidden)

			return
		}

		fmt.Printf("debug:x email=(%s)\n", email)
	*/

	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "internal/template" // default value for local development
	}
	templatePath := filepath.Join(templateDir, "add_book.html")

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("template error: %v", err)
		redirectToErrorPageWithMessageAndStatusCode(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}

	now := time.Now()

	pageVariables := PageVariables{
		Year:     now.Format("2006"),
		SiteKey:  captcha.SiteKey,
		LoggedIn: false,
	}

	err = t.Execute(w, pageVariables)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("template error: %v", err)
		return
	}
}

func RemoveImage(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// TODO: check auth

	r.ParseForm()
	imageID := r.PostFormValue("image_id")

	log.Printf("debug:x about to remove=(%s)", imageID)

	_, err := db.Exec("DELETE FROM book_images WHERE image_id=$1", imageID)
	if err != nil {
		http.Error(w, "Error removing image", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Image removed OK..."))
}
