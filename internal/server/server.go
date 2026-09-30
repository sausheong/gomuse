// Package server is the Muse web app: a score editor that renders tunes to
// WAV files and gives each one a shareable page.
package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/xid"
	"gopkg.in/yaml.v3"

	"github.com/sausheong/gomuse/internal/muse"
	"github.com/sausheong/gomuse/scores"
	"github.com/sausheong/gomuse/web"
)

// maxSamplesPerChannel caps how many samples per channel the web app will
// synthesise, so a huge score can't be used to run the server out of memory.
// The command line tool has no such limit.
const maxSamplesPerChannel = 6000000

// maxRequestBody caps the size of a submitted score, again to keep the
// server's memory use bounded.
const maxRequestBody = 256 * 1024 // 256 KiB

// samplePattern is the strict set of characters allowed in a /sample/{name}.
var samplePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

// Server holds the parsed templates and the directories generated tunes and
// their scores are written to.
type Server struct {
	tunesDir  string // <data>/tunes, served at /static/tunes/
	scoresDir string // <data>/scores, read back by /share/{id}

	// editSecret signs the per-tune edit tokens handed out as cookies (see
	// editToken below), so it's generated fresh for each server run.
	editSecret []byte

	indexTmpl  *template.Template
	sampleTmpl *template.Template
	tuneTmpl   *template.Template
	shareTmpl  *template.Template
}

// New creates the tunes and scores directories under dataDir and parses the
// embedded templates. It fails if any template is broken or the
// directories can't be created.
func New(dataDir string) (*Server, error) {
	s := &Server{
		tunesDir:   filepath.Join(dataDir, "tunes"),
		scoresDir:  filepath.Join(dataDir, "scores"),
		editSecret: make([]byte, 32),
	}
	if _, err := rand.Read(s.editSecret); err != nil {
		return nil, err
	}
	for _, dir := range []string{s.tunesDir, s.scoresDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	parse := func(pages ...string) (*template.Template, error) {
		for i, p := range pages {
			pages[i] = "templates/" + p
		}
		return template.ParseFS(web.Templates, pages...)
	}
	var err error
	if s.indexTmpl, err = parse("index.html", "try.html", "links.html"); err != nil {
		return nil, err
	}
	if s.sampleTmpl, err = parse("sample.html", "try.html", "links.html"); err != nil {
		return nil, err
	}
	if s.tuneTmpl, err = parse("tune.html", "try.html", "links.html"); err != nil {
		return nil, err
	}
	if s.shareTmpl, err = parse("share.html", "links.html"); err != nil {
		return nil, err
	}
	return s, nil
}

// Handler returns the app's routes. Generated tunes keep their original
// /static/tunes/ URLs so existing share links keep working.
func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err) // the embedded directory is fixed at build time
	}

	router := http.NewServeMux()
	router.HandleFunc("GET /{$}", s.index)
	router.HandleFunc("GET /sample/{name}", s.sample)
	router.HandleFunc("POST /create", s.create)
	router.HandleFunc("GET /share/{id}", s.share)
	router.Handle("GET /static/tunes/", http.StripPrefix("/static/tunes/", http.FileServer(http.Dir(s.tunesDir))))
	router.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return router
}

// front page
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if err := s.indexTmpl.Execute(w, nil); err != nil {
		log.Printf("cannot render index page - %v", err)
	}
}

// show a sample score
func (s *Server) sample(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !samplePattern.MatchString(name) || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	score, err := fs.ReadFile(scores.FS, name+".yaml")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.sampleTmpl.Execute(w, string(score)); err != nil {
		log.Printf("cannot render sample page - %v", err)
	}
}

// editToken returns the per-tune token that proves the caller is the one
// this server handed a tune id to, rather than someone who merely knows (or
// has guessed) that id from its public share link. The share ids from
// github.com/rs/xid are sequential/predictable, so the id itself is not
// enough to authorise overwriting an existing tune.
func (s *Server) editToken(id string) string {
	mac := hmac.New(sha256.New, s.editSecret)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// setEditCookie gives the caller the token for the tune it just created or
// re-created, scoped to /create only, so it's never sent to the public
// /share or /static routes.
func (s *Server) setEditCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "edit_" + id,
		Value:    s.editToken(id),
		Path:     "/create",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ownsTune reports whether the request carries the edit cookie this server
// handed out for id, i.e. whether the caller was actually given this id
// rather than guessing it.
func (s *Server) ownsTune(r *http.Request, id string) bool {
	c, err := r.Cookie("edit_" + id)
	return err == nil && hmac.Equal([]byte(c.Value), []byte(s.editToken(id)))
}

// create the wav file given the score
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// get the score
	score := r.PostFormValue("score")

	// id must either be blank (a new tune), or a valid, existing-format xid
	// whose edit cookie the caller holds. A syntactically valid id the
	// caller doesn't hold the cookie for is most likely a guessed public
	// share id rather than one this server gave out, so it's never reused -
	// a fresh id is minted instead, which means the tune it names can never
	// be overwritten this way.
	id := r.PostFormValue("guid")
	switch {
	case id == "":
		id = xid.New().String()
	default:
		if _, err := xid.FromString(id); err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if !s.ownsTune(r, id) {
			id = xid.New().String()
		}
	}
	s.setEditCookie(w, id)

	// error message if something goes wrong
	var message string

	// parse the score into a wave file
	var sc muse.Score
	name, err := muse.Parse(&sc, []byte(score), filepath.Join(s.tunesDir, id), maxSamplesPerChannel)
	if err != nil {
		// a failure writing the wav file (disk full, permissions, ...)
		// wraps an *fs.PathError that names the server's absolute install
		// path - show the visitor a generic message instead and log the
		// real one. Parse's own validation errors (bad note, unknown
		// envelope, tune too long, ...) never wrap a filesystem error, so
		// those are still shown verbatim.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			log.Printf("cannot create wav file for %s - %v", id, err)
			message = "could not create the tune, please try again"
		} else {
			message = err.Error()
		}
	}

	// write the score to a score file for sharing later
	if err := os.WriteFile(filepath.Join(s.scoresDir, id+".yaml"), []byte(score), 0644); err != nil {
		log.Printf("cannot write score file for %s - %v", id, err)
		if message == "" {
			message = "could not save the score, please try again"
		}
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Message  string
		Name     string
		Score    string
		Filename string
	}{id, message, name, score, "static/tunes/" + id + ".wav"}

	if err := s.tuneTmpl.Execute(w, &data); err != nil {
		log.Printf("cannot render tune page - %v", err)
	}
}

// share page for sharing to Facebook, Twitter etc
func (s *Server) share(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := xid.FromString(id); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// read from the score file, given the id
	score, err := os.ReadFile(filepath.Join(s.scoresDir, id+".yaml"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// unmarshal the score file into a Score struct
	var sc muse.Score
	if err := yaml.Unmarshal(score, &sc); err != nil {
		log.Printf("cannot unmarshal score file - %v", err)
		http.Error(w, "cannot read score", http.StatusInternalServerError)
		return
	}

	// anonymous struct to send back to the page
	data := struct {
		ID       string
		Name     string
		Score    string
		Filename string
	}{id, sc.Name, string(score), "/static/tunes/" + id + ".wav"}

	if err := s.shareTmpl.Execute(w, &data); err != nil {
		log.Printf("cannot render share page - %v", err)
	}
}

// String describes where the server keeps its generated files.
func (s *Server) String() string {
	return fmt.Sprintf("tunes in %s, scores in %s", s.tunesDir, s.scoresDir)
}
