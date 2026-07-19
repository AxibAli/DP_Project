// Package dashboard serves the server-rendered HTML pages. It reads
// animal data via the Store only to render the initial shell; all live
// updates happen client-side by polling the JSON API.
package dashboard

import (
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"livestock/internal/storage"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templatesFS embed.FS

type Dashboard struct {
	Store      *storage.Store
	indexTmpl  *template.Template
	animalTmpl *template.Template
}

func New(store *storage.Store) (*Dashboard, error) {
	indexTmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return nil, err
	}
	animalTmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/animal.html")
	if err != nil {
		return nil, err
	}
	return &Dashboard{Store: store, indexTmpl: indexTmpl, animalTmpl: animalTmpl}, nil
}

func (d *Dashboard) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", d.index)
	r.Get("/animals/{id}", d.animalDetail)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	return r
}

func (d *Dashboard) index(w http.ResponseWriter, r *http.Request) {
	if err := d.indexTmpl.ExecuteTemplate(w, "layout", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d *Dashboard) animalDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	animal, err := d.Store.GetAnimal(uint(id64))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Animal interface{}
	}{Animal: animal}

	if err := d.animalTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
