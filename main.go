package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/go-sql-driver/mysql"
	"github.com/thedevsaddam/renderer"
)

var rnd *renderer.Render
var db *sql.DB

const (
	dbDriver       = "mysql"
	dbDataSource   = "root:root@tcp(127.0.0.1:3306)/demo_todo1"
	collectionName = "todo"
	port           = ":9090"
)

type (
	todoModel struct {
		ID        int       `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"created_at"`
	}
	todo struct {
		ID        int       `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"created_at"`
	}
)

func init() {
	rnd = renderer.New()
	var err error
	db, err = sql.Open(dbDriver, dbDataSource)
	checkErr(err)
	createTodoTable()
}

func createTodoTable() {
	query := `
		CREATE TABLE IF NOT EXISTS todo (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			completed BOOLEAN,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := db.Exec(query)
	checkErr(err)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodo(w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}
	rows, err := db.Query("SELECT id, title, completed, created_at FROM todo")
	checkErr(err)
	defer rows.Close()

	for rows.Next() {
		var t todoModel
		err := rows.Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt)
		checkErr(err)
		todos = append(todos, t)
	}

	todoList := []todo{}
	for _, t := range todos {
		todoList = append(todoList, todo{
			ID:        t.ID,
			Title:     t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to decode request body",
			"error":   err,
		})
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title is required",
		})
		return
	}

	result, err := db.Exec("INSERT INTO todo (title, completed, created_at) VALUES (?, ?, ?)", t.Title, t.Completed, time.Now())
	checkErr(err)

	lastInsertID, err := result.LastInsertId()
	checkErr(err)

	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "Todo created successfully",
		"todo_id": lastInsertID,
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	stmt, err := db.Prepare("DELETE FROM todo WHERE id=?")
	checkErr(err)

	result, err := stmt.Exec(id)
	checkErr(err)

	rowsAffected, err := result.RowsAffected()
	checkErr(err)

	if rowsAffected == 0 {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Todo not found",
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to decode request body",
			"error":   err,
		})
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title field is required",
		})
		return
	}

	stmt, err := db.Prepare("UPDATE todo SET title=?, completed=? WHERE id=?")
	checkErr(err)

	result, err := stmt.Exec(t.Title, t.Completed, id)
	checkErr(err)

	rowsAffected, err := result.RowsAffected()
	checkErr(err)

	if rowsAffected == 0 {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Todo not found",
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo updated successfully",
	})
}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Println("Listening on port", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Println("Listen:", err)
		}
	}()
	<-stopChan
	log.Println("Shutting down servers.........")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped...")
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodo)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
