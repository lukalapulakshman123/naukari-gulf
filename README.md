# naukari-gulf

# Book Management API

## Overview

The **Book Management API** is a RESTful service built with Golang and Gin that allows users to manage books in a library. It supports operations such as retrieving, adding, updating, and deleting books, leveraging Redis for caching and Kafka for event-driven messaging.

## Features

- Fetch all books or a specific book by ID
- Add a new book
- Update an existing book
- Delete a book
- Redis caching for optimized performance
- Kafka integration for event-based messaging
- Swagger documentation for API reference

## Technologies Used

- **Golang** (Gin Framework)
- **PostgreSQL** (via GORM ORM)
- **Redis** (for caching)
- **Kafka** (for messaging)
- **Swagger** (API documentation)
- **Logrus** (logging)

## Installation & Setup

### Prerequisites

Ensure you have the following installed:

- Go 1.18+
- PostgreSQL
- Redis
- Kafka
- Git

### Clone the Repository

```sh
git clone https://github.com/lukalapulakshman123/naukari-gulf.git
cd naukari-gulf/server
```

### Install Dependencies

```sh
go mod tidy
```

### Run the Server

```sh
go run main.go
```

### Access Swagger Documentation

Once the server is running, access Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

## API Endpoints

### Get Books

**GET** `/books`

```sh
curl -X GET "http://localhost:8080/books" -H "Accept: application/json"
```

### Get a Book by ID

**GET** `/books?id={book_id}`

```sh
curl -X GET "http://localhost:8080/books?id=123" -H "Accept: application/json"
```

### Add a Book

**POST** `/books`

```sh
curl -X POST "http://localhost:8080/books" \
     -H "Content-Type: application/json" \
     -d '{"title": "Golang Basics", "author": "John Doe", "year": 2023}'
```

### Update a Book

**PUT** `/books/{id}`

```sh
curl -X PUT "http://localhost:8080/books/123" \
     -H "Content-Type: application/json" \
     -d '{"title": "Advanced Golang"}'
```

### Delete a Book

**DELETE** `/books/{id}`

```sh
curl -X DELETE "http://localhost:8080/books/123"
```
