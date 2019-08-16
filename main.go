package main

import (
	"context"
	"fmt"

	"encoding/json"
	"log"
	"net/http"
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

type codedetail struct {
	Usecase  string `bson:"usecase,omitempty"`
	Rank     int    `bson:"rank,omitempty"`
	Compiled bool   `bson:"compiled"`
	Homepage string `bson:"homepage,omitempty"`
	Download string `bson:"download,omitempty"`
	Votes    int    `bson:"votes"`
}

type language struct {
	Name   string     `bson:"name,omitempty"`
	Detail codedetail `bson:"codedetail,omitempty"`
}

var c *mongo.Client

func createlanguage(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	var detail codedetail
	_ = json.NewDecoder(req.Body).Decode(&detail)
	name := strings.ToLower(params["name"])

	lang := language{name, detail}

	id := InsertNewLanguage(c, lang)

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
	var langmap = make(map[string]*codedetail)
	langs := ReturnAllLanguages(c, bson.M{})
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

	lang := ReturnOneLanguage(c, bson.M{"name": name})
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

	languagesRemoved := RemoveOneLanguage(c, bson.M{"name": name})

	_ = json.NewEncoder(w).Encode(fmt.Sprintf("{'count' : %d}", languagesRemoved))

	return
}

func voteonlanguage(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	name := strings.ToLower(params["name"])

	fmt.Println("incoming vote for: " + name)

	votesUpdated := UpdateVote(c, bson.M{"name": name})

	_ = json.NewEncoder(w).Encode(fmt.Sprintf("{'count' : %d}", votesUpdated))
}

func ReturnAllLanguages(client *mongo.Client, filter bson.M) []*language {
	var langs []*language
	collection := client.Database("languages").Collection("test")
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

func ReturnOneLanguage(client *mongo.Client, filter bson.M) *language {
	var lang language
	collection := client.Database("languages").Collection("test")
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

func InsertNewLanguage(client *mongo.Client, lang language) interface{} {
	collection := client.Database("languages").Collection("test")
	insertResult, err := collection.InsertOne(context.TODO(), lang)
	if err != nil {
		log.Fatalln("Error on inserting new language", err)
		return nil
	}
	return insertResult.InsertedID
}

func RemoveOneLanguage(client *mongo.Client, filter bson.M) int64 {
	collection := client.Database("languages").Collection("test")
	deleteResult, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		log.Fatal("Error on deleting one Hero", err)
	}
	return deleteResult.DeletedCount
}

func UpdateVote(client *mongo.Client, filter bson.M) int64 {
	collection := client.Database("languages").Collection("test")
	updatedData := bson.M{"$inc": bson.M{"codedetail.votes": 1}}
	updatedResult, err := collection.UpdateOne(context.TODO(), filter, updatedData)
	if err != nil {
		log.Fatal("Error on updating one Hero", err)
	}
	return updatedResult.ModifiedCount
}

//GetClient returns a MongoDB Client
func GetClient() *mongo.Client {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
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
	c = GetClient()
	err := c.Ping(context.Background(), readpref.Primary())
	if err != nil {
		log.Fatal("Couldn't connect to the database", err)
	} else {
		log.Println("Connected!")
	}
}

func main() {
	fmt.Println("serving on port 8080!!")

	router := mux.NewRouter()

	router.HandleFunc("/languages/{name}", createlanguage).Methods("POST")
	router.HandleFunc("/languages", getlanguages).Methods("GET")
	router.HandleFunc("/languages/{name}", getlanguagebyname).Methods("GET")
	router.HandleFunc("/languages/{name}", deletelanguagebyname).Methods("DELETE")
	router.HandleFunc("/languages/{name}/vote", voteonlanguage).Methods("GET")

	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET, POST"})

	log.Fatal(http.ListenAndServe(":8080", handlers.CORS(originsOk, headersOk, methodsOk)(router)))
}
