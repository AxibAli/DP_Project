// Package dashboard serves the server-rendered HTML pages. It reads
// animal data via the Store only to render the initial shell; all live
// updates happen client-side by polling the JSON API. Every page here
// re-validates the session against the database (via auth.Service) and
// redirects unauthenticated visitors to /login — the same rule the JSON
// API enforces, just expressed as a redirect instead of a 401 body.
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

	"livestock/internal/auth"
	"livestock/internal/middleware"
	"livestock/internal/models"
	"livestock/internal/storage"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templatesFS embed.FS

type Dashboard struct {
	Store      *storage.Store
	Auth       *auth.Service
	indexTmpl  *template.Template
	animalTmpl *template.Template
	loginTmpl  *template.Template
	usersTmpl  *template.Template
}

func New(store *storage.Store, authSvc *auth.Service) (*Dashboard, error) {
	indexTmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return nil, err
	}
	animalTmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/animal.html")
	if err != nil {
		return nil, err
	}
	loginTmpl, err := template.ParseFS(templatesFS, "templates/login.html")
	if err != nil {
		return nil, err
	}
	usersTmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/users.html")
	if err != nil {
		return nil, err
	}
	return &Dashboard{
		Store: store, Auth: authSvc,
		indexTmpl: indexTmpl, animalTmpl: animalTmpl, loginTmpl: loginTmpl, usersTmpl: usersTmpl,
	}, nil
}

func (d *Dashboard) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/login", d.loginPage)
	r.Get("/", d.index)
	r.Get("/animals/{id}", d.animalDetail)
	r.Get("/admin/users", d.usersPage)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	return r
}

// requireUser validates the session cookie the same way the JSON API's
// Authenticate middleware does. On failure it redirects to /login and
// returns ok=false so the caller can simply `return`.
func (d *Dashboard) requireUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, false
	}
	user, _, err := d.Auth.ValidateSession(cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, false
	}
	return user, true
}

func (d *Dashboard) loginPage(w http.ResponseWriter, r *http.Request) {
	// Already-authenticated visitors don't need the login form.
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil {
		if _, _, err := d.Auth.ValidateSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	if err := d.loginTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type pageData struct {
	User *models.User
}

func (d *Dashboard) index(w http.ResponseWriter, r *http.Request) {
	user, ok := d.requireUser(w, r)
	if !ok {
		return
	}
	if err := d.indexTmpl.ExecuteTemplate(w, "layout", pageData{User: user}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d *Dashboard) usersPage(w http.ResponseWriter, r *http.Request) {
	user, ok := d.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != models.RoleAdmin {
		http.Error(w, "403 Forbidden: admin access required", http.StatusForbidden)
		return
	}
	if err := d.usersTmpl.ExecuteTemplate(w, "layout", pageData{User: user}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type animalPageData struct {
	pageData
	Animal *models.Animal
}

func (d *Dashboard) animalDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := d.requireUser(w, r)
	if !ok {
		return
	}

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
	if user.Role != models.RoleAdmin && animal.OwnerID != user.ID {
		http.Error(w, "403 Forbidden: you do not have access to this animal", http.StatusForbidden)
		return
	}

	data := animalPageData{pageData: pageData{User: user}, Animal: animal}
	if err := d.animalTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
