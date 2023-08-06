package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(w).Encode(v)
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type ApiError struct {
	Error string
}

func MakeHTTPHandlerFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := f(w, r)

		if err != nil {
			// handle error
			WriteJSON(w, http.StatusOK, err.Error)
		}
	}
}

type APIServer struct {
	addr     string
	redisCli *RedisCli

	apiURL string
	apiKey string
}

func NewAPIServer(addr string, redisCli *RedisCli) *APIServer {
	apiKey, err := GetEnv("OPENAI_KEY")
	if err != nil {
		log.Fatal(err)
	}

	// Set up the API endpoint
	apiURL, err := GetEnv("OPENAI_URL")
	if err != nil {
		log.Fatal(err)
	}

	return &APIServer{
		addr:     addr,
		redisCli: redisCli,

		apiKey: apiKey,
		apiURL: apiURL,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()

	// Balance routes
	router.HandleFunc("/balance/{id}", MakeHTTPHandlerFunc(s.handleBalanceGet))
	router.HandleFunc("/balance-topup/{id}/{amount}", MakeHTTPHandlerFunc(s.handleBalanceTopUpRequest))
	router.HandleFunc("/balance-deduct/{id}/{amount}", MakeHTTPHandlerFunc(s.handleBalanceDeductRequest))

	// OpenAI routes
	router.HandleFunc("/chatgpt", MakeHTTPHandlerFunc(s.handleChatGPTProxyRequest))

	log.Println("Server running on: ", s.addr)

	http.ListenAndServe(s.addr, router)
}

func (s *APIServer) handleBalanceGet(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}

	idStr := mux.Vars(r)["id"]
	fmt.Println(idStr)

	idInt, err := strconv.Atoi(idStr)
	if err != nil {
		return err
	}

	balanceStr, err := s.redisCli.GetValueByKey(fmt.Sprintf("user-%s", idStr))
	if err != nil {
		return err
	}

	balanceFloat, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return err
	}

	user := User{
		ID:      idInt,
		Balance: balanceFloat,
	}

	WriteJSON(w, http.StatusOK, user)

	return nil
}

func (s *APIServer) handleBalanceTopUpRequest(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]

	amount := mux.Vars(r)["amount"]

	return s.updateUserBalance(id, amount, true)
}

func (s *APIServer) handleBalanceDeductRequest(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]

	amount := mux.Vars(r)["amount"]

	return s.updateUserBalance(id, amount, false)
}

func (s *APIServer) handleChatGPTProxyRequest(w http.ResponseWriter, r *http.Request) error {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("Error reading request body: %d", http.StatusInternalServerError)
	}

	// Check key in headers
	s.checkHeaderKey(r.Header)

	payload := map[string]interface{}{
		"prompt":      string(requestBody),
		"model":       "gpt-3.5-turbo",
		"temperature": 1.0,
		"max_tokens":  100,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Error creating request payload: %d", http.StatusInternalServerError)
	}

	req, err := http.NewRequest("POST", s.apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("Error creating API request: %d", http.StatusInternalServerError)
	}

	// Copy request headers from the client's request
	CopyHeaders(req.Header, r.Header)

	// Set the OpenAI API key in the Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Content-Type", "application/json")

	// Send the request to OpenAI API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error sending API request: %d", http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading API response: %d", http.StatusInternalServerError)
	}

	w.WriteHeader(resp.StatusCode)

	// Copy OpenAI API response to client's response
	_, err = w.Write(responseBody)
	if err != nil {
		return fmt.Errorf("Error writing response: %d", http.StatusInternalServerError)
	}

	return nil
}

// checkHeaderKey checks if user's service-key is stored in redis.
func (s *APIServer) checkHeaderKey(src http.Header) error {
	for key, values := range src {
		for _, value := range values {
			if key == "service-key" {
				_, err := s.redisCli.GetValueByKey(fmt.Sprintf("key-%s", value))
				if err != nil {
					return err
				}

				return nil
			}
		}
	}

	return errors.New("service-key header not found")
}

func (s *APIServer) updateUserBalance(id, amount string, asc bool) error {
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return err
	}

	balanceStr, err := s.redisCli.GetValueByKey(fmt.Sprintf("user-%s", id))
	if err != nil {
		return err
	}

	balanceFloat, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return err
	}

	var newBalance float64
	if asc {
		newBalance = balanceFloat + amountFloat
	} else {
		newBalance = balanceFloat - amountFloat
	}

	err = s.redisCli.SetValueByKey(fmt.Sprintf("user-%s", id), fmt.Sprintf("%f", newBalance))
	if err != nil {
		return err
	}

	return nil
}
