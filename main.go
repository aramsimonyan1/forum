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

// Post structure
type Post struct {
	ID         string
	Title      string
	Content    string
	Category   string
	CreatedAt  time.Time
	Comments   []Comment
	IsLoggedIn bool // to check whether the user is logged in or not
}

// User structure
type User struct {
	ID       string
	Username string
	Password string
}

// Comment structure
type Comment struct {
	ID        string
	PostID    string
	Content   string
	CreatedAt time.Time
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create posts table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			title TEXT,
			content TEXT,
			category TEXT,
			created_at TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Create users table
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

	// Create comments table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			post_id TEXT,
			content TEXT,
			created_at TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
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
		http.Error(w, "Email already taken", http.StatusConflict)
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
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
			http.Error(w, "Incorrect email", http.StatusUnauthorized)
		} else {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Compare the provided password with the hashed password
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		http.Error(w, "Incorrect password", http.StatusUnauthorized)
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

	// Redirect to the home page
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

func emailExists(email string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		log.Println(err)
		return true // Assume email exists in case of an error
	}
	return count > 0
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve user ID from the cookie
		cookie, err := r.Cookie("forum-session")
		if err != nil || cookie.Value == "" {
			// User is not authenticated, redirect to login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// User is authenticated, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

// getPostsFromDatabase retrieves all posts from the database
func getPostsFromDatabase() ([]Post, error) {
	var posts []Post

	rows, err := db.Query(`
		SELECT id, title, content, category, created_at
		FROM posts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt)
		if err != nil {
			return nil, err
		}

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
		SELECT id, post_id, content, created_at
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
		err := rows.Scan(&comment.ID, &comment.PostID, &comment.Content, &comment.CreatedAt)
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

	// Create routes
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/post/", viewPostHandler)
	http.HandleFunc("/like/", likePostHandler)
	http.HandleFunc("/dislike/", dislikePostHandler)
	http.HandleFunc("/create-post", createPostHandler)

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in
	cookie, err := r.Cookie("forum-session")
	isLoggedIn := err == nil && cookie.Value != ""

	// Debugging: Print the IsLoggedIn value
	fmt.Println("IsLoggedIn:", isLoggedIn)

	// Retrieve posts and comments for display
	posts, err := getPostsFromDatabase()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Retrieve comments for each post
	for i := range posts {
		comments, err := getCommentsForPost(posts[i].ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		posts[i].Comments = comments
	}

	// Display posts and comments to the user
	tmpl, err := template.ParseFiles("templates/home.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, struct {
		IsLoggedIn bool
		Posts      []Post
	}{
		IsLoggedIn: isLoggedIn,
		Posts:      posts,
	})
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
	category := r.Form.Get("category")

	// Generate a unique ID for the post
	postID := uuid.New().String()

	// Insert the post into the database
	_, err = db.Exec(`
		INSERT INTO posts (id, title, content, category, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, postID, title, content, category, time.Now())
	if err != nil {
		log.Printf("Error inserting post into the database: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to the home page
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

	// Implement the logic to increment the like count in the database
	// (you may need a separate table to store likes and dislikes)

	// Redirect back to the post page
	http.Redirect(w, r, fmt.Sprintf("/post/%s", postID), http.StatusSeeOther)
}

func dislikePostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	postID := extractPostID(r.URL.Path)

	// Implement the logic to increment the dislike count in the database
	// (you may need a separate table to store likes and dislikes)

	// Redirect back to the post page
	http.Redirect(w, r, fmt.Sprintf("/post/%s", postID), http.StatusSeeOther)
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

func getPosts() ([]Post, error) {
	rows, err := db.Query(`
		SELECT id, title, content, category, created_at
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
		err := rows.Scan(&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// ...
func getPostByID(postID string) (*Post, error) {
	var post Post
	err := db.QueryRow(`
		SELECT id, title, content, category, created_at
		FROM posts
		WHERE id = ?
	`, postID).Scan(&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &post, nil
}
