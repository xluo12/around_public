package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"gopkg.in/olivere/elastic.v3"
	"net/http"
	"reflect"
	"strings"
	"time"
)


const (
	TYPE_USER = "user"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func checkUser(username, password string) bool {
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		fmt.Printf("ES is not setup %v\n", err)
		return false
	}

	// Search with a term query
	termQuery := elastic.NewTermQuery("username", strings.ToLower(username))
	queryResult, err := es_client.Search().
		Index(INDEX).
		Query(termQuery).
		Pretty(true).
		Do()
	if err != nil {
		fmt.Printf("ES query failed %v\n", err)
		return false
	}

	var tyu User
	for _, item := range queryResult.Each(reflect.TypeOf(tyu)) {
		u := item.(User)

		return u.Password == password && u.Username == username
	}
	// If no user exist, return false.
	return false
}

// Add a new user. Return true if successfully.
func addUser(username, password string) bool {
	// In theory, BigTable is a better option for storing user credentials than ES. However,
	// since BT is more expensive than ES so usually students will disable BT.
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		fmt.Printf("ES is not setup %v\n", err)
		return false
	}

	user := &User{
		Username: username,
		Password: password,
	}

	// Search with a term query
	termQuery := elastic.NewTermQuery("username", strings.ToLower(username))
	queryResult, err := es_client.Search().
		Index(INDEX).
		Query(termQuery).
		Pretty(true).
		Do()
	if err != nil {
		fmt.Printf("ES query failed %v\n", err)
		return false
	}

	if queryResult.TotalHits() > 0 {
		fmt.Printf("User %s has existed, cannot create duplicate user.\n", username)
		return false
	}

	// Save it to index
	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE_USER).
		Id(username).
		BodyJson(user).
		Refresh(true).
		Do()
	if err != nil {
		fmt.Printf("ES save failed %v\n", err)
		return false
	}

	return true
}

// If signup is successful, a new session is created.
func signupHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one signup request")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain")

	decoder := json.NewDecoder(r.Body)
	var u User
	if err := decoder.Decode(&u); err != nil {
		m := fmt.Sprintf("Failed to parse body %v", r.Body)
		fmt.Println(m)
		http.Error(w, m, http.StatusBadRequest)
		return
	}

	if u.Username != "" && u.Password != "" {
		if addUser(u.Username, u.Password) {
			fmt.Println("User added successfully.")
			w.Write([]byte("User added successfully."))
		} else {
			fmt.Println("Username existed")
			http.Error(w, "Username already existed.", http.StatusInternalServerError)
		}
	} else {
		fmt.Println("Empty password or username.")
		http.Error(w, "Empty password or username", http.StatusInternalServerError)
	}
}

// If login is successful, a new token is created.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one login request")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain")

	decoder := json.NewDecoder(r.Body)
	var u User
	if err := decoder.Decode(&u); err != nil {
		m := fmt.Sprintf("Failed to parse body %v", r.Body)
		fmt.Println(m)
		http.Error(w, m, http.StatusBadRequest)
		return
	}

	if checkUser(u.Username, u.Password) {
		token := jwt.New(jwt.SigningMethodHS256)
		claims := token.Claims.(jwt.MapClaims)
		/* Set token claims */
		claims["username"] = u.Username
		claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

		/* Sign the token with our secret */
		tokenString, _ := token.SignedString(mySigningKey)

		/* Finally, write the token to the browser window */
		w.Write([]byte(tokenString))
	} else {
		fmt.Println("Invalid password or username.")
		http.Error(w, "Invalid password or username", http.StatusForbidden)
	}
}