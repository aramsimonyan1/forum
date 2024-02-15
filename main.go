package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

type User struct {
	ID       string
	Username string
	Password string
}

type Post struct {
	ID            string
	Title         string
	Content       string
	Categories    []string
	CreatedAt     time.Time
	LikesCount    int
	DislikesCount int
	Comments      []Comment
	IsLoggedIn    bool
}

type PostInteraction struct {
	UserID string
	PostID string
	Action string
}

type Comment struct {
	ID            string
	PostID        string
	Content       string
	CreatedAt     time.Time
	LikesCount    int
	DislikesCount int
}

type CommentInteraction struct {
	UserID    string
	CommentID string
	Action    string
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT,
			username TEXT,
			password TEXT
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			title TEXT,
			content TEXT,
			categories TEXT,
			created_at TIMESTAMP,
			likes_count INT DEFAULT 0,
            dislikes_count INT DEFAULT 0
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS post_interactions (
			user_id TEXT,
			post_id TEXT,
			action TEXT,
			PRIMARY KEY (user_id, post_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (post_id) REFERENCES posts(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			post_id TEXT,
			content TEXT,
			created_at TIMESTAMP,
			likes_count INT DEFAULT 0,
            dislikes_count INT DEFAULT 0,
            FOREIGN KEY (post_id) REFERENCES posts(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS comment_interactions (
        user_id TEXT,
        comment_id TEXT,
        action TEXT,
        PRIMARY KEY (user_id, comment_id),
        FOREIGN KEY (user_id) REFERENCES users(id),
        FOREIGN KEY (comment_id) REFERENCES comments(id)
    )
`)
	if err != nil {
		log.Fatal(err)
	}
}

// Function retrieves the user ID from the given HTTP request
func getUserID(r *http.Request) string { // Takes an http.Request object (r) as its parameter. This object represents an incoming HTTP request.
	cookie, err := r.Cookie("forum-session") // Attempt to retrieve the "forum-session" cookie from the request
	if err != nil {                          // if there is an error retrieving the cookie (e.g. the cookie is not present or there is some issue accessing it.)
		return "" // Return an empty string (this indicates that the user ID could not be retrieved from the cookie)
	}
	return cookie.Value // return the value (user ID stored in the "forum-session" cookie)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Register handler received a request")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	fmt.Println("Form values:", r.Form)

	// Retrieve form data
	email := r.FormValue("email")
	username := r.FormValue("username")
	password := r.FormValue("password")

	fmt.Printf("Received data - Email: %s, Username: %s, Password: %s\n", email, username, password)

	// Check if email is empty
	if email == "" {
		http.Error(w, "Email cannot be empty", http.StatusBadRequest)
		return
	}

	// Check if email is already taken
	if emailExists(email) {
		errorMessage := "This email is already registered in database. Use your password to login."
		// Display an error message and redirect after 5 seconds.  //%s is a placeholder replaced by the value of errorMessage variable
		errorPage := fmt.Sprintf(`
            <html>
				<body style="font-size: 25px;">
                    <p>%s</p>                                        
                    <meta http-equiv="refresh" content="5;url=/">
                </body>
            </html>
        `, errorMessage)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(errorPage))
		return
	}

	// Function takes password coverted into a byte slice and the cost factor, to securely hash a password using the bcrypt algorithm to be stored in the database.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost) // bcrypt.DefaultCost is a constant provided by the bcrypt package, representing
	if err != nil {                                                                          // the default cost factor that determines how computationally expensive the hash function is.
		log.Println(err) // If there was an error, during the hashing process it logs the error using log.Println(err) and
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return // returns an HTTP 500 Internal Server Error response using http.Error.
	}

	// Insert the user into the database
	userID := uuid.New().String()
	_, err = db.Exec(`
        INSERT INTO users (id, email, username, password)
        VALUES (?, ?, ?, ?)
    `, userID, email, username, string(hashedPassword))
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Println("Successfully registered a new user")

	// Redirect to the home page to login
	http.Redirect(w, r, "/home?message=Registration%20successful", http.StatusSeeOther)
}

func emailExists(email string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		log.Println(err)
		return true // Assume email exists in case of an error
	}
	return count > 0
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Retrieve form data
	email := r.Form.Get("email")
	password := r.Form.Get("password")

	// Retrieve user from the database
	var userID, hashedPassword string
	err = db.QueryRow(`
		SELECT id, password
		FROM users
		WHERE email = ?
	`, email).Scan(&userID, &hashedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			errorMessage := "Incorrect email. Redirecting to the main page..."
			// Display an error message and redirect after 4 seconds
			errorPage := fmt.Sprintf(`
				<html>
					<body style="font-size: 2em;">
						<p>%s</p>
						<meta http-equiv="refresh" content="4;url=/">
					</body>
				</html>
			`, errorMessage)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(errorPage))
		} else {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Compare the provided password with the hashed password
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		errorMessage := "Invalid password. Please try again."
		// Display an error message and redirect after 5 seconds
		errorPage := fmt.Sprintf(`
            <html>
				<body>
					<p style="font-size: 2em;">%s</p>
                    <meta http-equiv="refresh" content="5;url=/">
                </body>
            </html>
        `, errorMessage)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(errorPage))
		return
	}

	// After successful login, create a new session ID and set it in a cookie
	sessionID := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:    "forum-session",
		Value:   sessionID,
		Expires: time.Now().Add(24 * time.Hour), // Set expiration time
		Path:    "/",
	})

	// Redirect to the home page after successful login
	http.Redirect(w, r, "/?message=Login%20successful", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Expire the cookie to delete the session
	http.SetCookie(w, &http.Cookie{
		Name:    "forum-session",
		Value:   "",
		Expires: time.Now(),
		Path:    "/",
	})

	// Redirect to the home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve user ID from the cookie
		cookie, err := r.Cookie("forum-session")
		if err != nil || cookie.Value == "" {
			// User is not authenticated, redirect to home page
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// User is authenticated, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

// getPostsFromDatabase retrieves all posts from the database
func getPostsFromDatabase(categoryFilter string) ([]Post, error) {
	var posts []Post

	var query string
	if categoryFilter != "" {
		query = `
            SELECT id, title, content, categories, created_at, likes_count, dislikes_count
            FROM posts
            WHERE ? IN (categories)
            ORDER BY created_at DESC
        `
	} else {
		query = `
            SELECT id, title, content, categories, created_at, likes_count, dislikes_count
            FROM posts
            ORDER BY created_at DESC
        `
	}

	rows, err := db.Query(query, categoryFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		var categoriesString string
		err := rows.Scan(&post.ID, &post.Title, &post.Content, &categoriesString, &post.CreatedAt, &post.LikesCount, &post.DislikesCount)
		if err != nil {
			return nil, err
		}

		// Split the categories string into a slice
		post.Categories = strings.Split(categoriesString, ",")

		// Retrieve comments for the post
		comments, err := getCommentsForPost(post.ID)
		if err != nil {
			return nil, err
		}
		post.Comments = comments

		posts = append(posts, post)
	}

	return posts, nil
}

// getCommentsForPost retrieves all comments for a specific post from the database
func getCommentsForPost(postID string) ([]Comment, error) {
	var comments []Comment

	rows, err := db.Query(`
		SELECT id, post_id, content, created_at, likes_count, dislikes_count
		FROM comments
		WHERE post_id = ?
		ORDER BY created_at DESC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var comment Comment
		err := rows.Scan(&comment.ID, &comment.PostID, &comment.Content, &comment.CreatedAt, &comment.LikesCount, &comment.DislikesCount)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}
	return comments, nil
}

func main() {
	// Initialize the database
	initDB()

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Create routes
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/create-post", createPostHandler)
	http.HandleFunc("/add-comment/", addCommentHandler)
	http.HandleFunc("/post/", viewPostHandler)
	http.HandleFunc("/like/", likePostHandler)
	http.HandleFunc("/dislike/", dislikePostHandler)
	http.HandleFunc("/like-comment/", likeCommentHandler)
	http.HandleFunc("/dislike-comment/", dislikeCommentHandler)
	http.HandleFunc("/filter", categoryFilterHandler)

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in
	cookie, err := r.Cookie("forum-session")
	isLoggedIn := err == nil && cookie.Value != ""

	// Debugging: Print the IsLoggedIn value
	fmt.Println("IsLoggedIn:", isLoggedIn)

	// Check if the request contains category filter parameters
	categoryFilter := r.FormValue("category")

	// Retrieve posts and comments for display
	posts, err := getPostsFromDatabase(categoryFilter)
	if err != nil {
		log.Printf("Error getting posts from the database: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Retrieve comments for each post
	for i := range posts {
		comments, err := getCommentsForPost(posts[i].ID)
		if err != nil {
			log.Printf("Error getting comments for post %s: %v", posts[i].ID, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		posts[i].Comments = comments
	}

	// Display posts and comments to the user
	tmpl, err := template.ParseFiles("templates/home.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Pass additional data to the template, such as selected category for display
	tmplData := struct {
		IsLoggedIn       bool
		Posts            []Post
		SelectedCategory string
	}{
		IsLoggedIn:       isLoggedIn,
		Posts:            posts,
		SelectedCategory: categoryFilter,
	}

	if err := tmpl.Execute(w, tmplData); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in
	cookie, err := r.Cookie("forum-session")
	if err != nil || cookie.Value == "" {
		// User is not logged in, redirect to the login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// User is logged in, proceed with post creation

	// Parse the form data
	err = r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form data: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Retrieve form data
	title := r.Form.Get("title")
	content := r.Form.Get("content")
	categories := r.Form["categories[]"]

	// Generate a unique ID for the post
	postID := uuid.New().String()

	// Insert the post into the database
	_, err = db.Exec(`
		INSERT INTO posts (id, title, content, categories, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, postID, title, content, strings.Join(categories, ","), time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Printf("Error inserting post into the database: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to the home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func categoryFilterHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to home page with category filter parameters
	http.Redirect(w, r, "/?category="+r.FormValue("category"), http.StatusSeeOther)
}

func addCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in
	cookie, err := r.Cookie("forum-session")
	if err != nil || cookie.Value == "" {
		// User is not logged in, redirect to the login page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// User is logged in, proceed with comment creation

	// Parse the form data
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Retrieve form data
	// postID := r.Form.Get("postID")
	// Extract post ID from the URL path and the content from the form
	postID := extractPostID(r.URL.Path)
	content := r.Form.Get("commentContent")

	// Insert the comment into the database
	_, err = db.Exec(`
        INSERT INTO comments (id, post_id, content, created_at)
        VALUES (?, ?, ?, ?)
    `, uuid.New().String(), postID, content, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect back to the home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func viewPostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	postID := extractPostID(r.URL.Path)

	// Retrieve post details from the database
	post, err := getPostByID(postID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Render the post page
	tmpl, err := template.ParseFiles("templates/post.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, post)
}

func likePostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	postID := extractPostID(r.URL.Path)

	// Check if the user already disliked the post, reverse the interaction if true
	if hasUserInteractedWithPost(getUserID(r), postID, "dislike") {
		decreasePostDislikeCount(postID)
		removePostInteraction(getUserID(r), postID)
	} else if !hasUserInteractedWithPost(getUserID(r), postID, "like") {
		// Increment the like count and add the interaction only if the user has not liked the post before
		increasePostLikeCount(postID)
		addPostInteraction(getUserID(r), postID, "like")
	}

	// Redirect back to the home page with an anchor to the updated post
	http.Redirect(w, r, "/#post-"+postID, http.StatusSeeOther)
}

func hasUserInteractedWithPost(userID, postID, action string) bool {
	var count int
	err := db.QueryRow(`
        SELECT COUNT(*)
        FROM post_interactions
        WHERE user_id = ? AND post_id = ? AND action = ?
    `, userID, postID, action).Scan(&count)
	return err == nil && count > 0
}

func increasePostLikeCount(postID string) {
	_, err := db.Exec(`
        UPDATE posts
        SET likes_count = likes_count + 1
        WHERE id = ?
    `, postID)
	if err != nil {
		log.Println(err)
	}
}

func decreasePostLikeCount(postID string) {
	_, err := db.Exec(`
        UPDATE posts
        SET likes_count = likes_count - 1
        WHERE id = ? AND likes_count > 0
    `, postID)
	if err != nil {
		log.Println(err)
	}
}

func addPostInteraction(userID, postID, action string) {
	_, err := db.Exec(`
        INSERT INTO post_interactions (user_id, post_id, action)
        VALUES (?, ?, ?)
    `, userID, postID, action)
	if err != nil {
		log.Println(err)
	}
}

func removePostInteraction(userID, postID string) {
	_, err := db.Exec(`
        DELETE FROM post_interactions
        WHERE user_id = ? AND post_id = ?
    `, userID, postID)
	if err != nil {
		log.Println(err)
	}
}

func dislikePostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	postID := extractPostID(r.URL.Path)

	// Check if the user already liked the post, reverse the interaction if true
	if hasUserInteractedWithPost(getUserID(r), postID, "like") {
		decreasePostLikeCount(postID)
		removePostInteraction(getUserID(r), postID)
	} else if !hasUserInteractedWithPost(getUserID(r), postID, "dislike") {
		// Increment the dislike count and add the interaction only if the user has not disliked the post before
		increasePostDislikeCount(postID)
		addPostInteraction(getUserID(r), postID, "dislike")
	}

	// Redirect back to the home page with an anchor to the updated post
	http.Redirect(w, r, "/#post-"+postID, http.StatusSeeOther)
}

func increasePostDislikeCount(postID string) {
	_, err := db.Exec(`
        UPDATE posts
        SET dislikes_count = dislikes_count + 1
        WHERE id = ?
    `, postID)
	if err != nil {
		log.Println(err)
	}
}

func decreasePostDislikeCount(postID string) {
	_, err := db.Exec(`
        UPDATE posts
        SET dislikes_count = dislikes_count - 1
        WHERE id = ? AND dislikes_count > 0
    `, postID)
	if err != nil {
		log.Println(err)
	}
}

// extractPostID extracts the post ID from the URL path
func extractPostID(path string) string {
	// Assuming the URL path is in the format "/post/{id}" or "/like/{id}" or "/dislike/{id}"
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

func likeCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve comment ID from the URL
	commentID := extractCommentID(r.URL.Path)

	// Check if the user already disliked the comment, reverse the interaction if true
	if hasUserInteractedWithComment(getUserID(r), commentID, "dislike") {
		decreaseCommentDislikeCount(commentID)
		removeCommentInteraction(getUserID(r), commentID)
	} else if !hasUserInteractedWithComment(getUserID(r), commentID, "like") {
		// Increment the like count and add the interaction only if the user has not liked the comment before
		increaseCommentLikeCount(commentID)
		addCommentInteraction(getUserID(r), commentID, "like")
	}

	// Redirect back to the home page or the post page, depending on your design
	http.Redirect(w, r, "/#comment-"+commentID, http.StatusSeeOther)
}

func dislikeCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve comment ID from the URL
	commentID := extractCommentID(r.URL.Path)

	// Check if the user already liked the comment, reverse the interaction if true
	if hasUserInteractedWithComment(getUserID(r), commentID, "like") {
		decreaseCommentLikeCount(commentID)
		removeCommentInteraction(getUserID(r), commentID)
	} else if !hasUserInteractedWithComment(getUserID(r), commentID, "dislike") {
		// Increment the dislike count and add the interaction only if the user has not disliked the comment before
		increaseCommentDislikeCount(commentID)
		addCommentInteraction(getUserID(r), commentID, "dislike")
	}

	// Redirect back to the home page or the post page, depending on your design
	http.Redirect(w, r, "/#comment-"+commentID, http.StatusSeeOther)
}

// extractCommentID extracts the post ID from the URL path
func extractCommentID(path string) string {
	// Assuming the URL path is in the format "/post/{id}" or "/like/{id}" or "/dislike/{id}"
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// Update function to check if the user has interacted with a comment
func hasUserInteractedWithComment(userID, commentID, action string) bool {
	var count int
	err := db.QueryRow(`
        SELECT COUNT(*)
        FROM comment_interactions
        WHERE user_id = ? AND comment_id = ? AND action = ?
    `, userID, commentID, action).Scan(&count)
	return err == nil && count > 0
}

// Update function to increase comment like count
func increaseCommentLikeCount(commentID string) {
	_, err := db.Exec(`
        UPDATE comments
        SET likes_count = likes_count + 1
        WHERE id = ?
    `, commentID)
	if err != nil {
		log.Println(err)
	}
}

// Update function to decrease comment like count
func decreaseCommentLikeCount(commentID string) {
	_, err := db.Exec(`
        UPDATE comments
        SET likes_count = likes_count - 1
        WHERE id = ? AND likes_count > 0
    `, commentID)
	if err != nil {
		log.Println(err)
	}
}

// Update function to increase comment dislike count
func increaseCommentDislikeCount(commentID string) {
	_, err := db.Exec(`
        UPDATE comments
        SET dislikes_count = dislikes_count + 1
        WHERE id = ?
    `, commentID)
	if err != nil {
		log.Println(err)
	}
}

// Update function to decrease comment dislike count
func decreaseCommentDislikeCount(commentID string) {
	_, err := db.Exec(`
        UPDATE comments
        SET dislikes_count = dislikes_count - 1
        WHERE id = ? AND dislikes_count > 0
    `, commentID)
	if err != nil {
		log.Println(err)
	}
}

// Update function to add comment interaction
func addCommentInteraction(userID, commentID, action string) {
	_, err := db.Exec(`
        INSERT INTO comment_interactions (user_id, comment_id, action)
        VALUES (?, ?, ?)
    `, userID, commentID, action)
	if err != nil {
		log.Println(err)
	}
}

// Update function to remove comment interaction
func removeCommentInteraction(userID, commentID string) {
	_, err := db.Exec(`
        DELETE FROM comment_interactions
        WHERE user_id = ? AND comment_id = ?
    `, userID, commentID)
	if err != nil {
		log.Println(err)
	}
}

func getPosts() ([]Post, error) {
	rows, err := db.Query(`
		SELECT id, title, content, categories, created_at, likes_count, dislikes_count
		FROM posts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var categoriesString string
		err := rows.Scan(&post.ID, &post.Title, &post.Content, &categoriesString, &post.CreatedAt, &post.LikesCount, &post.DislikesCount)
		if err != nil {
			return nil, err
		}
		// Convert the comma-separated string to a slice of strings
		post.Categories = splitCategories(categoriesString)
		posts = append(posts, post)
	}

	return posts, nil
}

// splitCategories splits a comma-separated string into a slice of strings
func splitCategories(categoriesString string) []string {
	return strings.Split(categoriesString, ",")
}

// ...
func getPostByID(postID string) (*Post, error) {
	var post Post
	var categoriesString string
	err := db.QueryRow(`
		SELECT id, title, content, categories, created_at, likes_count, dislikes_count
		FROM posts
		WHERE id = ?
	`, postID).Scan(&post.ID, &post.Title, &post.Content, &categoriesString, &post.CreatedAt, &post.LikesCount, &post.DislikesCount)
	if err != nil {
		return nil, err
	}

	// Convert the comma-separated string to a slice of strings
	post.Categories = splitCategories(categoriesString)

	return &post, nil
}
