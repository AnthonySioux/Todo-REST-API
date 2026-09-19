package handlers

import (
	"net/http"
	"strconv"
	"todo_api/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateTodoInput struct {
	Title     string `json:"title" binding:"required"`
	Completed bool   `json:"completed"`
}

type UpdateTodoInput struct {
	Title *string `json:"title"`
	// &true ---------------> set completed as -> true
	// &false ---------------> set completed as -> false
	// nil ---------------> set completed as -> not provided
	Completed *bool `json:"completed"`
}

// BUG: this handler writes a response on the repository error path and then
// continues. c.JSON does not end the handler.
func CreateTodoHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")

		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found in context"})
			return
		}

		userID := userIDInterface.(string)

		var input CreateTodoInput

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		todo, err := repository.CreateTodo(pool, input.Title, input.Completed, userID)

		// BUG 4: on an insert failure this sends a 500 and then falls through
		// to the 201 below with todo == nil, appending "null" to the body.
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			// ✅ NEW CODE
			return
		}

		c.JSON(http.StatusCreated, todo)
	}
}

func GetAllTodosHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")

		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found in context"})
			return
		}
		// interface{} or any{},
		userID := userIDInterface.(string)

		todos, err := repository.GetAllTodos(pool, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, todos)
	}
}

// BUG: the catch-all error branch in this handler does not stop execution.
// Compare it with the ErrNoRows branch directly above it, which does.
func GetToDoByIDHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")

		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found in context"})
			return
		}
		// interface{} or any{},
		userID := userIDInterface.(string)

		idStr := c.Param("id")
		// "2" ------------> 2, nil
		// "a" ------------> 0, error ("invalid syntax")

		id, err := strconv.Atoi(idStr)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
			return
		}

		todo, err := repository.GetToDoByID(pool, id, userID)

		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
				return
			}

			// BUG 5: any error that is not ErrNoRows sends a 500 here and then
			// reaches the 200 below with todo == nil.
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			// ✅ NEW CODE
			return
		}

		c.JSON(http.StatusOK, todo)
	}
}

// BUG: the ID validation branch here rejects the input and then keeps going.
// The same branch in GetToDoByIDHandler stops correctly.
func UpdateToDoHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")

		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found in context"})
			return
		}
		// interface{} or any{},
		userID := userIDInterface.(string)

		idStr := c.Param("id")

		id, err := strconv.Atoi(idStr)

		// BUG 3: a non-numeric ID sends a 400 and then continues with id == 0,
		// so the body is still bound and a lookup still runs against the
		// database.
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
			// ✅ NEW CODE
			return
		}

		var input UpdateTodoInput

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if input.Title == nil && input.Completed == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one field (title or completed) must be provided"})
			return
		}

		existing, err := repository.GetToDoByID(pool, id, userID)

		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		title := existing.Title
		if input.Title != nil {
			title = *input.Title
		}

		completed := existing.Completed
		if input.Completed != nil {
			completed = *input.Completed
		}

		todo, err := repository.UpdateToDo(pool, id, title, completed, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, todo)

	}
}

// BUG: two branches in this handler write a response without returning, so a
// single request can produce three JSON objects and still run a DELETE query.
func DeleteToDoHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")

		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user_id not found in context"})
			return
		}
		// interface{} or any{},
		userID := userIDInterface.(string)

		idStr := c.Param("id")

		id, err := strconv.Atoi(idStr)

		// BUG 1: a non-numeric ID sends a 400 and then continues with id == 0,
		// so the DELETE below executes against the database anyway.
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
			// ✅ NEW CODE
			return
		}

		err = repository.DeleteToDo(pool, id, userID)

		if err != nil {
			if err.Error() == "todo with id "+idStr+" not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
				return
			}

			// BUG 2: sends a second body and then falls through to the success
			// message below, so the response reports a deletion that never
			// happened.
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			// ✅ NEW CODE
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Todo deleted successfully"})
	}
}
