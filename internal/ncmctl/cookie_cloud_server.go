package ncmctl

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

var (
	dataStore = make(map[string]string) // In-memory map to store data
	dataMutex = &sync.Mutex{}           // Mutex to handle concurrent access to the map
	apiRoot   = strings.TrimRight(getEnv("API_ROOT", ""), "/")
)

func StartCookieCloudServ() {
	http.HandleFunc(apiRoot+"/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World! API ROOT = %s", apiRoot)
	})

	http.HandleFunc(apiRoot+"/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Encrypted string `json:"encrypted"`
			UUID      string `json:"uuid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if body.Encrypted == "" || body.UUID == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Store the data in memory
		dataMutex.Lock()
		dataStore[body.UUID] = body.Encrypted
		dataMutex.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"action": "done"})
	})

	http.HandleFunc(apiRoot+"/get/", func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimPrefix(r.URL.Path, apiRoot+"/get/")
		if uuid == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		dataMutex.Lock()
		encrypted, exists := dataStore[uuid]
		dataMutex.Unlock()

		if !exists {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		password := r.FormValue("password")
		if password != "" {
			parsed, err := cookieDecrypt(uuid, encrypted, password)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(parsed)
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"encrypted": encrypted})
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})

	port := getEnv("PORT", "8088")
	log.Printf("Server started on http://localhost:%s%s", port, apiRoot)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func cookieDecrypt(uuid, encrypted, password string) (map[string]interface{}, error) {
	key := md5.Sum([]byte(uuid + "-" + password))
	block, err := aes.NewCipher(key[:16])
	if err != nil {
		return nil, err
	}

	encryptedBytes, err := hex.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	if len(encryptedBytes) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := encryptedBytes[:aes.BlockSize]
	encryptedBytes = encryptedBytes[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encryptedBytes, encryptedBytes)

	decrypted := strings.TrimRight(string(encryptedBytes), "\x00")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &parsed); err != nil {
		return nil, err
	}

	return parsed, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(strings.TrimRight(strings.Join(strings.Fields(strings.TrimSpace(key)), ""), "/"))
	if value == "" {
		return fallback
	}
	return value
}
