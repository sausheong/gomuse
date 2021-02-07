package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"gopkg.in/yaml.v2"
)

var serverFlag *bool
var dir string
var h string

func init() {
	serverFlag = flag.Bool("s", false, "start the Muse service")
	flag.Parse()
	var err error
	dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	h = dir + "/static/html/"
}

func main() {

	if *serverFlag {
		r := mux.NewRouter()
		r.HandleFunc("/", index)
		r.HandleFunc("/sample/{name}", sample)
		r.HandleFunc("/create", create)
		r.HandleFunc("/share/{id}", share)

		r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(dir+"/static"))))

		srv := &http.Server{
			Handler:      r,
			Addr:         "0.0.0.0:8888",
			WriteTimeout: 30 * time.Second,
			ReadTimeout:  30 * time.Second,
		}
		fmt.Println("Starting Muse server at", srv.Addr)
		log.Fatal(srv.ListenAndServe())

	} else {
		if len(os.Args) > 1 {
			arg := os.Args[1]
			t1 := time.Now()

			var s Score
			name, err := ParseFile(&s, arg)
			if err != nil {
				fmt.Printf("Cannot parse score file - %v", err)
			}
			dur := time.Since(t1)
			fmt.Println("Created tune", name, "in", arg+".wav", "in", dur.String())
		} else {
			fmt.Println("No score provided")
		}
	}
}

// front page
func index(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"index.html", h+"try.html", h+"links.html")
	t.Execute(w, nil)
}

// show a sample score
func sample(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"sample.html", h+"try.html", h+"links.html")
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/scores/" + vars["name"] + ".yaml")
	t.Execute(w, string(score))
}

// create the wav file given the score
func create(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"tune.html", h+"try.html", h+"links.html")
	r.ParseForm()
	// get the score
	score := r.PostFormValue("score")
	// error message if something goes wrong
	var message string

	// parse the score into a wave file
	id := r.PostFormValue("guid")
	if id == "" {
		guid := xid.New()
		id = guid.String()
	}

	name, err := parseAndCreateWav("static/tunes/"+id, []byte(score))
	if err != nil {
		message = err.Error()
	}

	// write the score to a score file for sharing later
	err = ioutil.WriteFile(dir+"/static/scores/"+id+".yaml", []byte(score), 0644)
	if err != nil {
		message = err.Error()
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Message  string
		Name     string
		Score    string
		Filename string
	}{id, message, name, score, "static/tunes/" + id + ".wav"}
	t.Execute(w, &data)
}

// share page for sharing to Facebook, Twitter etc
func share(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(h+"share.html", h+"links.html")
	// read from the score file, given theID
	vars := mux.Vars(r)
	score, _ := ioutil.ReadFile(dir + "/static/scores/" + vars["id"] + ".yaml")

	// unmarshal the score file into a Score struct
	var s *Score
	err := yaml.Unmarshal(score, &s)
	if err != nil {
		log.Printf("Cannot unmarshal score file - %v", err)
		return
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Name     string
		Score    string
		Filename string
	}{vars["id"], s.Name, string(score), "/static/tunes/" + vars["id"] + ".wav"}
	t.Execute(w, &data)
}

// parse the score and save to a wav file
func parseAndCreateWav(outfile string, score []byte) (name string, err error) {
	var s Score
	name, err = Parse(&s, score, outfile)
	if err != nil {
		log.Printf("Cannot parse score file - %v", err)
	}
	return
}
