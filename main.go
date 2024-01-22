package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var store = sessions.NewCookieStore([]byte("your-secret-key"))

// Post structure
type Post struct {
	ID        string
	Title     string
	Content   string
	Category  string
	CreatedAt time.Time
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

func main() {
	// Initialize the database
	initDB()

	// Create routes
	r := mux.NewRouter()

	// Apply authMiddleware to routes that require authentication
	authenticatedRouter := r.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(authMiddleware)
	authenticatedRouter.HandleFunc("/create-post", createPostHandler).Methods("POST")

	// Other routes
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/register", registerHandler).Methods("POST")
	r.HandleFunc("/login", loginHandler).Methods("POST")
	r.HandleFunc("/logout", logoutHandler).Methods("GET")
	r.HandleFunc("/post/{id}", viewPostHandler).Methods("GET")
	r.HandleFunc("/like/{id}", likePostHandler).Methods("POST")
	r.HandleFunc("/dislike/{id}", dislikePostHandler).Methods("POST")

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", r))
}

// authMiddleware is a middleware that checks if the user is authenticated
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the session
		session, err := store.Get(r, "forum-session")
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Check if the user is authenticated
		userID, ok := session.Values["userID"].(string)
		if !ok || userID == "" {
			// User is not authenticated, redirect to login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// User is authenticated, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
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
	username := r.Form.Get("username")
	password := r.Form.Get("password")

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

	// Redirect to the home page or login page
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		} else {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Compare the provided password with the hashed password
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create a new session
	session, err := store.Get(r, "forum-session")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set user ID in the session
	session.Values["userID"] = userID

	// Save the session
	err = session.Save(r, w)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to the home page or user-specific page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Delete the session
	session, err := store.Get(r, "forum-session")
	if err == nil {
		session.Options.MaxAge = -1
		err = session.Save(r, w)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Redirect to the home page or login page
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

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in
	session, err := store.Get(r, "forum-session")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check if the user is logged in
	userID, ok := session.Values["userID"].(string)
	if ok && userID != "" {
		// User is logged in, display post creation form or other user-specific content
		// ...

		// For now, let's redirect to the post creation form
		http.Redirect(w, r, "/create-post", http.StatusSeeOther)
		return
	}

	// User is not logged in, display login and registration options
	tmpl, err := template.ParseFiles("templates/login.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is logged in (you may need to implement user authentication)
	// For simplicity, this example assumes the user is logged in.

	// Parse the form data
	err := r.ParseForm()
	if err != nil {
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
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to the home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func viewPostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	vars := mux.Vars(r)
	postID := vars["id"]

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
	vars := mux.Vars(r)
	postID := vars["id"]

	// Implement the logic to increment the like count in the database
	// (you may need a separate table to store likes and dislikes)

	// Redirect back to the post page
	http.Redirect(w, r, fmt.Sprintf("/post/%s", postID), http.StatusSeeOther)
}

func dislikePostHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve post ID from the URL
	vars := mux.Vars(r)
	postID := vars["id"]

	// Implement the logic to increment the dislike count in the database
	// (you may need a separate table to store likes and dislikes)

	// Redirect back to the post page
	http.Redirect(w, r, fmt.Sprintf("/post/%s", postID), http.StatusSeeOther)
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
