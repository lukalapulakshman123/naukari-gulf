package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	model "myproject/dbUtils/tables"
	utils "myproject/utils"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	// Swagger docs
	_ "myproject/server/docs"

	files "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Book Management API
// @version         1.0
// @description     API for managing books in a library.
// @termsOfService  http://swagger.io/terms/
// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io
// @host      localhost:8080
// @BasePath  /api

type Service struct {
	Config        *viper.Viper
	Logger        *logrus.Logger
	Db            *gorm.DB
	HttpPort      int
	RedisClient   *redis.Client
	KafkaProducer *kafka.Producer
}

type KafkaMsg struct {
	Method  string
	Message model.Books
}

// bookRequest represents the fields allowed for updating a book
type bookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
}

var ctx = context.Background()

// Initialize services
func initService() *Service {
	service := &Service{
		Logger:        utils.GetLogger(),
		Db:            utils.GetDb(),
		HttpPort:      8080,
		RedisClient:   utils.GetRedisClient(),
		KafkaProducer: utils.GetKafkaProducer(),
	}
	return service
}

// Graceful shutdown handler
func handleShutdown(service *Service) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigchan
		service.Logger.Info("Shutting down gracefully...")
		service.KafkaProducer.Close()
		service.Logger.Info("Shutdown complete")
		os.Exit(0)
	}()
}

// @Summary Get books (all or by ID)
// @Description Fetch all books or a specific book by ID
// @Tags books
// @Produce json
// @Param id query string false "Book ID"
// @Success 200 {object} model.Books
// @Router /books [get]
func (service *Service) getBooks(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		var books []model.Books
		if utils.GetCache("all_books", &books, service.RedisClient, ctx) {
			c.JSON(http.StatusOK, books)
			return
		}

		if err := service.Db.Find(&books).Error; err != nil {
			service.Logger.Error("Failed to fetch books: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books"})
			return
		}

		utils.SetCache("all_books", books, 10*time.Minute, service.RedisClient, ctx)
		c.JSON(http.StatusOK, books)
		return
	}

	// Fetch a single book by ID
	var book model.Books
	if utils.GetCache("book:"+id, &book, service.RedisClient, ctx) {
		c.JSON(http.StatusOK, book)
		return
	}
	if err := service.Db.First(&book, "id = ?", id).Error; err != nil {
		service.Logger.Error("Book not found: ", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}
	utils.SetCache("book:"+id, book, 10*time.Minute, service.RedisClient, ctx)
	c.JSON(http.StatusOK, book)
}

// @Summary Add a new book
// @Tags books
// @Accept json
// @Produce json
// @Param book body bookRequest true "Book details"
// @Success 201 {object} model.Books
// @Router /books [post]
func (service *Service) addBook(c *gin.Context) {
	var book model.Books
	if err := c.ShouldBindJSON(&book); err != nil {
		service.Logger.Error("Invalid input: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	book.ID = uuid.New().String()
	if err := service.Db.Create(&book).Error; err != nil {
		service.Logger.Error("Error while creating the book: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Invalidate cache & send Kafka message
	service.RedisClient.Del(ctx, "all_books")
	utils.ProduceMessage("books", service.KafkaProducer, &KafkaMsg{
		Method:  "POST",
		Message: book,
	})

	c.JSON(http.StatusCreated, book)
}

// @Summary Update a book
// @Tags books
// @Accept json
// @Produce json
// @Param id path string true "Book ID"
// @Param book body bookRequest true "Updated book details"
// @Success 200 {object} model.Books
// @Router /books/{id} [put]
func (service *Service) updateBook(c *gin.Context) {
	id := c.Param("id")

	// Check if the book exists
	var book model.Books
	if err := service.Db.First(&book, "id = ?", id).Error; err != nil {
		service.Logger.Errorf("Book with id %s not found", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Bind updated JSON data
	var updateData model.Books
	if err := c.ShouldBindJSON(&updateData); err != nil {
		service.Logger.Error("Invalid input: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Update book in the database
	service.Db.Model(&book).Updates(updateData)

	// Invalidate cache & send Kafka message
	service.RedisClient.Del(ctx, "all_books", "book:"+id)
	utils.ProduceMessage("books", service.KafkaProducer, &KafkaMsg{
		Method:  "PUT",
		Message: book,
	})

	c.JSON(http.StatusOK, book)
}

// @Summary Delete a book
// @Tags books
// @Produce json
// @Param id path string true "Book ID"
// @Success 200 {object} map[string]string "Book deleted successfully"
// @Router /books/{id} [delete]

func (service *Service) deleteBook(c *gin.Context) {
	id := c.Param("id")

	// Check if the book exists
	var book model.Books
	if err := service.Db.First(&book, "id = ?", id).Error; err != nil {
		service.Logger.WithFields(logrus.Fields{
			"bookID": id,
			"error":  err.Error(),
		}).Error("Book not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Delete the book
	if err := service.Db.Delete(&book).Error; err != nil {
		service.Logger.WithFields(logrus.Fields{
			"bookID": id,
			"error":  err.Error(),
		}).Error("Failed to delete book")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book"})
		return
	}

	// Invalidate cache & send Kafka message
	service.RedisClient.Del(ctx, "all_books", "book:"+id)
	utils.ProduceMessage("books", service.KafkaProducer, &KafkaMsg{
		Method:  "DELETE",
		Message: book,
	})
	c.JSON(http.StatusOK, gin.H{"message": "Book deleted successfully"})
}

func main() {
	service := initService()
	handleShutdown(service)

	router := gin.Default()
	router.GET("/swagger/*any", ginSwagger.WrapHandler(files.Handler))
	router.GET("/books", service.getBooks)
	router.POST("/books", service.addBook)
	router.PUT("/books/:id", service.updateBook)
	router.DELETE("/books/:id", service.deleteBook) // Add delete endpoint

	router.Run(":8080")
}
