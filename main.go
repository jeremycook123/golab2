package main

import (
	"context"
	"fmt"
	"os"

	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	//including gorilla mux and handlers packages for HTTP routing and CORS support
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	//connections to mongo
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const MONGO_DB = "langdb"
const MONGO_COLLECTION = "languages"
const MONGO_DEFAULT_CONN_STR = "mongodb://mongo-0.mongo,mongo-1.mongo,mongo-2.mongo:27017/langdb"
const MONGO_DEFAULT_USERNAME = "admin"
const MONGO_DEFAULT_PASSWORD = "password"

type codedetail struct {
	Usecase  string `json:"usecase,omitempty" bson:"usecase"`
	Rank     int    `json:"rank,omitempty" bson:"rank"`
	Compiled bool   `json:"compiled" bson:"compiled"`
	Homepage string `json:"homepage,omitempty" bson:"homepage"`
	Download string `json:"download,omitempty" bson:"download"`
	Votes    int    `json:"votes" bson:"votes"`
}

type language struct {
	Name   string     `json:"name,omitempty" bson:"name"`
	Detail codedetail `json:"codedetail,omitempty" bson:"codedetail"`
}

var c *mongo.Client

func createlanguage(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	var detail codedetail
	_ = json.NewDecoder(req.Body).Decode(&detail)
	name := strings.ToLower(params["name"])

	fmt.Println(fmt.Sprintf("POST api call made to /languages/%s", name))

	lang := language{name, detail}

	id := insertNewLanguage(c, lang)

	if id == nil {
		_ = json.NewEncoder(w).Encode("{'result' : 'insert failed!'}")
	} else {
		err := json.NewEncoder(w).Encode(detail)
		if err != nil {
			http.Error(w, err.Error(), 400)
		}
	}

	return
}

func getlanguages(w http.ResponseWriter, _ *http.Request) {
	fmt.Println("GET api call made to /languages")

	var langmap = make(map[string]*codedetail)

	langs := returnAllLanguages(c, bson.M{})

	for _, lang := range langs {
		langmap[lang.Name] = &lang.Detail
	}

	err := json.NewEncoder(w).Encode(langmap)
	if err != nil {
		http.Error(w, err.Error(), 400)
	}
	return
}

func getlanguagebyname(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	name := strings.ToLower(params["name"])

	fmt.Println(fmt.Sprintf("GET api call made to /languages/%s", name))

	lang := returnOneLanguage(c, bson.M{"name": name})

	if lang == nil {
		_ = json.NewEncoder(w).Encode("{'result' : 'language not found'}")
	} else {
		err := json.NewEncoder(w).Encode(*lang)
		if err != nil {
			http.Error(w, err.Error(), 400)
		}
	}

	return
}

func deletelanguagebyname(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	name := strings.ToLower(params["name"])

	fmt.Println(fmt.Sprintf("DELETE api call made to /languages/%s", name))

	languagesRemoved := removeOneLanguage(c, bson.M{"name": name})

	_ = json.NewEncoder(w).Encode(fmt.Sprintf("{'count' : %d}", languagesRemoved))

	return
}

func voteonlanguage(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	name := strings.ToLower(params["name"])

	fmt.Println(fmt.Sprintf("GET api call made to /languages/%s/vote", name))

	//votesUpdated := updateVote(c, bson.M{"name": name})
	vchan := voteChannel()
	vchan <- name
	votesUpdated := <-vchan
	close(vchan)

	_ = json.NewEncoder(w).Encode(fmt.Sprintf("{'count' : %s}", votesUpdated))
}

func voteChannel() (vchan chan string) {
	vchan = make(chan string)

	go func() {
		name := <-vchan
		//fmt.Println(fmt.Sprintf("name is %s", name))
		votesUpdated := strconv.FormatInt((updateVote(c, bson.M{"name": name})), 10)
		vchan <- votesUpdated
	}()

	return vchan
}

func returnAllLanguages(client *mongo.Client, filter bson.M) []*language {
	var langs []*language
	collection := client.Database(MONGO_DB).Collection(MONGO_COLLECTION)

	cur, err := collection.Find(context.TODO(), filter)

	if err != nil {
		log.Fatal("Error on Finding all the documents", err)
	}
	for cur.Next(context.TODO()) {
		var lang language
		err = cur.Decode(&lang)
		if err != nil {
			log.Fatal("Error on Decoding the document", err)
		}
		langs = append(langs, &lang)
	}
	return langs
}

func returnOneLanguage(client *mongo.Client, filter bson.M) *language {
	var lang language
	collection := client.Database(MONGO_DB).Collection(MONGO_COLLECTION)

	singleResult := collection.FindOne(context.TODO(), filter)

	if singleResult.Err() == mongo.ErrNoDocuments {
		return nil
	}
	if singleResult.Err() != nil {
		log.Println("Find error: ", singleResult.Err())
	}
	singleResult.Decode(&lang)
	return &lang
}

func insertNewLanguage(client *mongo.Client, lang language) interface{} {
	collection := client.Database(MONGO_DB).Collection(MONGO_COLLECTION)

	insertResult, err := collection.InsertOne(context.TODO(), lang)

	if err != nil {
		log.Fatalln("Error on inserting new language", err)
		return nil
	}
	return insertResult.InsertedID
}

func removeOneLanguage(client *mongo.Client, filter bson.M) int64 {
	collection := client.Database(MONGO_DB).Collection(MONGO_COLLECTION)

	deleteResult, err := collection.DeleteOne(context.TODO(), filter)

	if err != nil {
		log.Fatal("Error on deleting one Hero", err)
	}
	return deleteResult.DeletedCount
}

func updateVote(client *mongo.Client, filter bson.M) int64 {
	collection := client.Database(MONGO_DB).Collection(MONGO_COLLECTION)
	updatedData := bson.M{"$inc": bson.M{"codedetail.votes": 1}}

	updatedResult, err := collection.UpdateOne(context.TODO(), filter, updatedData)

	if err != nil {
		log.Fatal("Error on updating one Hero", err)
	}
	return updatedResult.ModifiedCount
}

//getClient returns a MongoDB Client
func getClient() *mongo.Client {
	mongoconnstr := getEnv("MONGO_CONN_STR", MONGO_DEFAULT_CONN_STR)
	mongousername := getEnv("MONGO_USERNAME", MONGO_DEFAULT_USERNAME)
	mongopassword := getEnv("MONGO_PASSWORD", MONGO_DEFAULT_PASSWORD)

	fmt.Println("MongoDB connection details:")
	fmt.Println("MONGO_CONN_STR:" + mongoconnstr)
	fmt.Println("MONGO_USERNAME:" + mongousername)
	fmt.Println("MONGO_PASSWORD:")
	fmt.Println("attempting mongodb backend connection...")

	clientOptions := options.Client().ApplyURI(mongoconnstr)
	clientOptions.Auth.Username = mongousername
	clientOptions.Auth.Password = mongopassword

	client, err := mongo.NewClient(clientOptions)

	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func init() {
	c = getClient()
	err := c.Ping(context.Background(), readpref.Primary())
	if err != nil {
		log.Fatal("couldn't connect to the database", err)
	} else {
		log.Println("connected!!")
	}
}

func ok(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "OK!")
	return
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func main() {
	fmt.Println("version 1.00")
	fmt.Println("serving on port 8080...")
	fmt.Println("tests:")
	fmt.Println("curl -s localhost:8080/ok")
	fmt.Println("curl -s localhost:8080/languages")
	fmt.Println("curl -s localhost:8080/languages | jq .")

	router := mux.NewRouter()

	//setup routes
	router.HandleFunc("/languages/{name}", createlanguage).Methods("POST")
	router.HandleFunc("/languages", getlanguages).Methods("GET")
	router.HandleFunc("/languages/{name}", getlanguagebyname).Methods("GET")
	router.HandleFunc("/languages/{name}", deletelanguagebyname).Methods("DELETE")
	router.HandleFunc("/languages/{name}/vote", voteonlanguage).Methods("GET")
	router.HandleFunc("/ok", ok).Methods("GET")

	//required for CORS - ajax API requests originating from the react browser vote app
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET, POST"})

	//listen on port 8080
	log.Fatal(http.ListenAndServe(":8080", handlers.CORS(originsOk, headersOk, methodsOk)(router)))
}
