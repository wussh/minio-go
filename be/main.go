package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var s3Client *minio.Client

type Response struct {
	Message string `json:"message"`
}

func init() {
	log.Println("Initializing the application...")

	// Load .env file
	log.Println("Loading environment variables from .env file")
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get credentials and endpoint from environment variables
	accessKey := os.Getenv("ACCESS_KEY")
	secretKey := os.Getenv("SECRET_KEY")
	s3Endpoint := os.Getenv("S3_ENDPOINT")

	if accessKey == "" || secretKey == "" || s3Endpoint == "" {
		log.Fatalf("Missing required environment variables: ACCESS_KEY, SECRET_KEY, or S3_ENDPOINT")
	}
	log.Printf("Environment variables loaded: ACCESS_KEY=%s, S3_ENDPOINT=%s", accessKey, s3Endpoint)

	// Initialize MinIO client
	log.Println("Initializing MinIO client")
	s3Client, err = minio.New(s3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}
	log.Println("MinIO client initialized successfully")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling file upload request")

	// Parse multipart form data
	log.Println("Parsing multipart form data")
	err := r.ParseMultipartForm(10 << 20) // Limit to 10 MB files
	if err != nil {
		log.Printf("Error parsing form data: %v", err)
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the bucket name
	bucketName := r.FormValue("bucket")
	if bucketName == "" {
		log.Println("Bucket name is missing in the request")
		http.Error(w, "Bucket name is required", http.StatusBadRequest)
		return
	}
	log.Printf("Received bucket name: %s", bucketName)

	// Get the file from the request
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from request: %v", err)
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("Received file: %s, size: %d bytes", fileHeader.Filename, fileHeader.Size)

	// Upload the file to the specified bucket
	objectName := fileHeader.Filename
	log.Printf("Uploading file to bucket %s with object name %s", bucketName, objectName)
	info, err := s3Client.PutObject(
		context.Background(),
		bucketName,
		objectName,
		file,
		fileHeader.Size,
		minio.PutObjectOptions{ContentType: fileHeader.Header.Get("Content-Type")},
	)
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
		http.Error(w, fmt.Sprintf("Failed to upload file: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("File uploaded successfully: %s to bucket %s, size: %d bytes", objectName, bucketName, info.Size)

	// Create a response message
	response := Response{
		Message: fmt.Sprintf("Uploaded %s to bucket %s, size: %d bytes", objectName, bucketName, info.Size),
	}

	// Set response header and send the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func listObjectsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling list objects request")

	// Get the bucket name from the query parameters
	bucketName := r.URL.Query().Get("bucket")
	if bucketName == "" {
		log.Println("Bucket name is missing in the request")
		http.Error(w, "Bucket name is required", http.StatusBadRequest)
		return
	}
	log.Printf("Received bucket name: %s", bucketName)

	// List objects in the specified bucket
	log.Printf("Listing objects in bucket %s", bucketName)
	objectCh := s3Client.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	// Create a response array to store object names
	var objects []string
	for object := range objectCh {
		if object.Err != nil {
			log.Printf("Error listing object: %v", object.Err)
			http.Error(w, fmt.Sprintf("Error listing objects: %v", object.Err), http.StatusInternalServerError)
			return
		}
		log.Printf("Found object: %s", object.Key)
		objects = append(objects, object.Key)
	}

	// Create the response message
	response := Response{
		Message: fmt.Sprintf("Listed %d objects in bucket %s", len(objects), bucketName),
	}

	// Set response header and send the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": response.Message,
		"objects": objects,
	})
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling file download request")

	// Get the bucket name and object name from query parameters
	bucketName := r.URL.Query().Get("bucket")
	objectName := r.URL.Query().Get("object")
	if bucketName == "" || objectName == "" {
		log.Println("Bucket or object name is missing in the request")
		http.Error(w, "Bucket and object names are required", http.StatusBadRequest)
		return
	}
	log.Printf("Received bucket name: %s, object name: %s", bucketName, objectName)

	// Retrieve the object from the specified bucket
	log.Printf("Downloading object %s from bucket %s", objectName, bucketName)
	object, err := s3Client.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Failed to download object: %v", err)
		http.Error(w, fmt.Sprintf("Failed to download object: %v", err), http.StatusInternalServerError)
		return
	}
	defer object.Close()

	// Set the response header for file download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", objectName))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Copy the object data to the response writer
	log.Printf("Streaming object data to response")
	_, err = io.Copy(w, object)
	if err != nil {
		log.Printf("Error streaming object data: %v", err)
		http.Error(w, fmt.Sprintf("Error streaming object data: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("Object downloaded successfully: %s", objectName)
}

func main() {
	log.Println("Starting the HTTP server")

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/list", listObjectsHandler)
	http.HandleFunc("/download", downloadHandler)

	// Enable CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"POST", "GET", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)(http.DefaultServeMux)

	port := ":8080"
	log.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(port, corsHandler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
