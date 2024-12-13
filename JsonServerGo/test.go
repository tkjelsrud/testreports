package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// StoredObject represents a stored object with an ID and arbitrary data
type StoredObject struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

// In-memory storage and mutex for thread safety
var (
	mu      sync.Mutex
	storage = make(map[string]StoredObject)
)

func main() {
	setupGracefulShutdown()

	storage, err := LoadAllObjects("data")
	if err != nil {
		fmt.Println("Error loading objects:", err)
	} else {
		fmt.Println("Loaded objects:", len(storage))
	}
	// Define routes
	http.HandleFunc("/objects/", objectHandler) // Handle /objects/{id}
	http.HandleFunc("/objects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getAllObjectsHandler(w, r)
		} else if r.Method == http.MethodPost {
			postObjectHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/status", statusHandler) // Handle /status

	// Start the server
	port := 8080
	fmt.Printf("Starting server on http://127.0.0.1:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func setupGracefulShutdown() {
	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine to handle shutdown
	go func() {
		<-stop // Wait for termination signal
		fmt.Println("\nShutting down...")

		// Save all objects before exiting
		mu.Lock()
		defer mu.Unlock()
		if err := SaveAllObjects("data", storage); err != nil {
			fmt.Println("Error saving objects:", err)
		} else {
			fmt.Println("Objects saved successfully.")
		}

		os.Exit(0)
	}()
}

// Handler to post a new object
func postObjectHandler(w http.ResponseWriter, r *http.Request) {
	var rawData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate a unique ID using uuid
	id := uuid.New().String()

	// Create the StoredObject
	storedObject := StoredObject{
		ID:   id,
		Data: rawData,
	}

	// Send the response immediately to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(storedObject)

	// Asynchronously process the object
	go func(obj StoredObject) {
		// Store the object in memory (this happens after the client gets a response)
		mu.Lock()
		storage[obj.ID] = obj
		mu.Unlock()

		// Simulate additional processing (e.g., logging, saving to a database, etc.)
		processObject(obj)
	}(storedObject)
}

// Simulate additional processing for the object
func processObject(obj StoredObject) {
	fmt.Printf("Processing object ID: %s\n", obj.ID)
	// Simulate a delay for processing
	time.Sleep(2 * time.Second)
	fmt.Printf("Processing completed for object ID: %s\n", obj.ID)
}

// Handler to get all objects or their IDs
func getAllObjectsHandler(w http.ResponseWriter, r *http.Request) {
	depth := r.URL.Query().Get("depth")

	mu.Lock()
	defer mu.Unlock()

	if depth == "full" {
		// Return full objects
		objects := make([]StoredObject, 0, len(storage))
		for _, obj := range storage {
			objects = append(objects, obj)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(objects)
	} else {
		// Default: return only GUIDs
		ids := make([]string, 0, len(storage))
		for id := range storage {
			ids = append(ids, id)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ids)
	}
}

// Handler to get a specific object by ID
func objectHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/objects/")

	mu.Lock()
	defer mu.Unlock()

	obj, exists := storage[id]
	if !exists {
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

// Handler to report server status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	mu.Lock()
	numItems := len(storage)
	mu.Unlock()

	status := map[string]interface{}{
		"memory_usage_mb": memStats.Alloc / 1024 / 1024,
		"num_items":       numItems,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func LoadAllObjects(folder string) (map[string]StoredObject, error) {
	storage := make(map[string]StoredObject)

	files, err := os.ReadDir(folder)
	if err != nil {
		return nil, fmt.Errorf("failed to read folder: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			id := file.Name()[:len(file.Name())-len(".json")] // Remove ".json" suffix
			data, err := os.ReadFile(filepath.Join(folder, file.Name()))
			if err != nil {
				fmt.Printf("Skipping file %s due to read error: %v\n", file.Name(), err)
				continue
			}

			var obj StoredObject
			if err := json.Unmarshal(data, &obj); err != nil {
				fmt.Printf("Skipping file %s due to JSON parsing error: %v\n", file.Name(), err)
				continue
			}

			storage[id] = obj
		}
	}

	return storage, nil
}

func SaveObject(folder string, id string, data StoredObject) error {
	filePath := filepath.Join(folder, fmt.Sprintf("%s.json", id))
	return os.WriteFile(filePath, data.Data, 0644)
}

// SaveAllObjects saves all objects in storage to files
func SaveAllObjects(folder string, storage map[string]StoredObject) error {
	// Ensure the folder exists
	if err := os.MkdirAll(folder, 0755); err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	for id, obj := range storage {
		filePath := fmt.Sprintf("%s/%s.json", folder, id)
		// Marshal the StoredObject into JSON
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal object %s: %w", id, err)
		}

		// Write the JSON data to the file
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed to save object %s: %w", id, err)
		}
	}
	return nil
}
