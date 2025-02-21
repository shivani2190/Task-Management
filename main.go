package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"github.com/joho/godotenv"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"github.com/gin-contrib/cors" // ✅ Import CORS package
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string             `bson:"username" json:"username"`
	Password string             `bson:"password" json:"password"`
}

type Task struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	AssignedTo  string             `bson:"assignedTo" json:"assignedTo"`
	Status      string             `bson:"status" json:"status"`
	Priority    string             `bson:"priority" json:"priority"`
}

var client *mongo.Client
var userCollection *mongo.Collection
var taskCollection *mongo.Collection

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Task)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI is not set in environment variables")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var errConn error
	client, errConn = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if errConn != nil {
		log.Fatal("Failed to connect to MongoDB Atlas:", errConn)
	}

	userCollection = client.Database("taskdb").Collection("users")
	taskCollection = client.Database("taskdb").Collection("tasks")
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()
	clients[ws] = true
	for {
		var task Task
		if err := ws.ReadJSON(&task); err != nil {
			log.Println("Error reading WebSocket message:", err)
			delete(clients, ws)
			break
		}
		broadcast <- task
	}
}

func handleMessages() {
	for {
		task := <-broadcast
		for client := range clients {
			err := client.WriteJSON(task)
			if err != nil {
				log.Println("WebSocket Send Error:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func getAIPriority(description string) (string, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	apiKey := os.Getenv("API_KEY")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash")
	prompt := []genai.Part{
		genai.Text("Determine the priority level (High, Medium, Low) for this task: " + description),
	}

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		return "", err
	}

	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					return string(textPart), nil
				}
			}
		}
	}
	return "Medium", nil
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var jwtKey = []byte("your_secret_key")

func generateJWT(username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func signup(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	user.Password = string(hashedPassword)
	_, err := userCollection.InsertOne(context.Background(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User created"})
}

func login(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var foundUser User
	err := userCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&foundUser)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(foundUser.Password), []byte(user.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	token, _ := generateJWT(user.Username)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func createTask(c *gin.Context) {
	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task.ID = primitive.NewObjectID()
	priority, err := getAIPriority(task.Description)
	if err == nil {
		task.Priority = priority
	} else {
		task.Priority = "Medium"
	}
	_, err = taskCollection.InsertOne(context.Background(), task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}
	broadcast <- task
	c.JSON(http.StatusOK, task)
}

func getTasks(c *gin.Context) {
	cursor, err := taskCollection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks"})
		return
	}
	var tasks []Task
	cursor.All(context.Background(), &tasks)
	c.JSON(http.StatusOK, tasks)
}

func updateTask(c *gin.Context) {
	id, _ := primitive.ObjectIDFromHex(c.Param("id"))
	var updatedTask Task
	if err := c.ShouldBindJSON(&updatedTask); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := taskCollection.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": bson.M{"status": updatedTask.Status}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Task updated"})
}



// Structs for API Request/Response
type TaskSuggestionRequest struct {
	Description string `json:"description"`
}

type TaskSuggestionResponse struct {
	Suggestions []string `json:"suggestions"`
}

// AI Function to Get Task Suggestions
func getAITaskSuggestions(description string) ([]string, error) {
	// Load API Key
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	apiKey := os.Getenv("API_KEY")

	// Initialize Gemini Client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Use Gemini Pro (Text Model)
	model := client.GenerativeModel("gemini-2.0-flash")

	// Create Prompt
	prompt := []genai.Part{
		genai.Text("Generate 3 detailed subtasks for: " + description),
	}

	// Generate AI Response
	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, err
	}

	// Parse Response
	var suggestions []string
	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok { // Correctly extract text
					suggestions = append(suggestions, string(textPart))
				}
			}
		}
	}

	return suggestions, nil
}

// API Route for AI Task Suggestions
func getAITaskSuggestionsHandler(c *gin.Context) {
	var request TaskSuggestionRequest

	// Validate JSON Input
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get AI Suggestions
	suggestions, err := getAITaskSuggestions(request.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get AI task suggestions"})
		return
	}

	// Send Response
	c.JSON(http.StatusOK, TaskSuggestionResponse{Suggestions: suggestions})
}


func main() {
	r := gin.Default()

	// ✅ Enable CORS with proper settings
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://yourfrontenddomain.com"}, // Update for your frontend
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	
	r.POST("/signup", signup)
	r.POST("/login", login)
	r.POST("/tasks", createTask)
	r.GET("/tasks", getTasks)
	r.PUT("/tasks/:id", updateTask)
	r.GET("/ws", func(c *gin.Context) {
		handleConnections(c.Writer, c.Request)
	})
	go handleMessages()

	r.POST("/ai-task-suggestions", getAITaskSuggestionsHandler)

	log.Fatal(r.Run(":8080"))
}
