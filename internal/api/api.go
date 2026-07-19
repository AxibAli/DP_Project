// Package api exposes the REST/JSON surface: authentication, admin user
// management, and CRUD over animals/readings/alerts/vaccinations. Role and
// ownership checks happen here (or in internal/middleware) — never on the
// client — so every response already reflects what the caller is allowed
// to see.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"livestock/internal/auth"
	"livestock/internal/middleware"
	"livestock/internal/models"
	"livestock/internal/storage"
)

type API struct {
	Store *storage.Store
	Auth  *auth.Service
	// SecureCookies controls the Secure flag on session/CSRF cookies. It
	// must be true whenever the app is served over HTTPS; it defaults to
	// false only to allow local http:// development/testing.
	SecureCookies bool
}

func New(store *storage.Store, authSvc *auth.Service, secureCookies bool) *API {
	return &API{Store: store, Auth: authSvc, SecureCookies: secureCookies}
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()

	// Public: no session exists yet, so there's nothing for CSRF's
	// double-submit check to compare against.
	r.Post("/auth/register", a.register)
	r.Post("/auth/login", a.login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(a.Auth))
		r.Use(middleware.CSRF()) // no-op for GET/HEAD/OPTIONS

		r.Post("/auth/logout", a.logout)
		r.Get("/auth/me", a.me)

		r.Route("/animals", func(r chi.Router) {
			r.Get("/", a.listAnimals)
			r.Post("/", a.createAnimal)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", a.getAnimal)
				r.Put("/", a.updateAnimal)
				r.Delete("/", a.deleteAnimal)
				r.Get("/readings", a.listReadings)
				r.Get("/alerts", a.listAlertsForAnimal)
				r.Get("/vaccinations", a.listVaccinations)
				r.Post("/vaccinations", a.createVaccination)
			})
		})

		r.Route("/alerts", func(r chi.Router) {
			r.Get("/", a.listActiveAlerts)
			r.Post("/{id}/resolve", a.resolveAlert)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.RequireRole(models.RoleAdmin))
			r.Get("/", a.listUsers)
			r.Post("/", a.createUser)
			r.Put("/{id}", a.updateUser)
			r.Delete("/{id}", a.deleteUser)
		})
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func idParam(r *http.Request) (uint, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func currentUser(r *http.Request) *models.User {
	user, _ := middleware.UserFromContext(r.Context())
	return user
}

// --- Auth ---

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Email == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Self-registration always creates a plain USER account. Admin
	// accounts are only ever created by an existing admin via /api/users,
	// so a client can never grant itself elevated privileges.
	user, err := a.Auth.Register(in.Email, in.Password, models.RoleUser)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, publicUser(user))
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Email == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, session, err := a.Auth.Login(in.Email, in.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	csrfToken, err := auth.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to establish session")
		return
	}

	a.setSessionCookies(w, session.SessionToken, csrfToken, session.ExpiresAt)
	writeJSON(w, http.StatusOK, publicUser(user))
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil {
		_ = a.Auth.Logout(cookie.Value)
	}
	a.clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, publicUser(currentUser(r)))
}

type publicUserView struct {
	ID        uint        `json:"id"`
	Email     string      `json:"email"`
	Role      models.Role `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
}

func publicUser(u *models.User) publicUserView {
	return publicUserView{ID: u.ID, Email: u.Email, Role: u.Role, CreatedAt: u.CreatedAt}
}

func (a *API) setSessionCookies(w http.ResponseWriter, sessionToken, csrfToken string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: false, // must be readable by JS to echo back in the CSRF header
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *API) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{middleware.SessionCookieName, middleware.CSRFCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: name == middleware.SessionCookieName,
			Secure:   a.SecureCookies,
			SameSite: http.SameSiteStrictMode,
		})
	}
}

// --- Users (admin only) ---

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.Store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]publicUserView, len(users))
	for i, u := range users {
		views[i] = publicUser(&u)
	}
	writeJSON(w, http.StatusOK, views)
}

type createUserInput struct {
	Email    string      `json:"email"`
	Password string      `json:"password"`
	Role     models.Role `json:"role"`
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	var in createUserInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Email == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if in.Role != models.RoleAdmin && in.Role != models.RoleUser {
		writeError(w, http.StatusBadRequest, "role must be ADMIN or USER")
		return
	}

	user, err := a.Auth.Register(in.Email, in.Password, in.Role)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, publicUser(user))
}

type updateUserInput struct {
	Role models.Role `json:"role"`
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in updateUserInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if in.Role != models.RoleAdmin && in.Role != models.RoleUser {
		writeError(w, http.StatusBadRequest, "role must be ADMIN or USER")
		return
	}
	if err := a.Store.UpdateUserRole(id, in.Role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	user, err := a.Store.GetUser(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, publicUser(user))
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if id == currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}
	if err := a.Store.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Animals ---

func (a *API) listAnimals(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var animals []models.Animal
	var err error
	if user.Role == models.RoleAdmin {
		animals, err = a.Store.ListAnimals()
	} else {
		animals, err = a.Store.ListAnimalsByOwner(user.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, animals)
}

func (a *API) createAnimal(w http.ResponseWriter, r *http.Request) {
	var in models.Animal
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// The owner is always the requesting user, regardless of anything the
	// client sends — ownership must never be client-assignable.
	in.ID = 0
	in.OwnerID = currentUser(r).ID
	in.CreatedAt = time.Now()
	if err := a.Store.CreateAnimal(&in); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

type animalWithLatest struct {
	models.Animal
	LatestReading *models.Reading `json:"latest_reading,omitempty"`
}

// loadOwnedAnimal fetches an animal and verifies the requesting user may
// access it: admins may access any animal, users only their own. It writes
// the appropriate error response and returns ok=false when access should
// be denied, so callers can just `return` on failure.
func (a *API) loadOwnedAnimal(w http.ResponseWriter, r *http.Request, id uint) (*models.Animal, bool) {
	animal, err := a.Store.GetAnimal(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "animal not found")
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	user := currentUser(r)
	if user.Role != models.RoleAdmin && animal.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "you do not have access to this animal")
		return nil, false
	}
	return animal, true
}

func (a *API) getAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	animal, ok := a.loadOwnedAnimal(w, r, id)
	if !ok {
		return
	}
	resp := animalWithLatest{Animal: *animal}
	if latest, err := a.Store.LatestReading(id); err == nil {
		resp.LatestReading = latest
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) updateAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, ok := a.loadOwnedAnimal(w, r, id)
	if !ok {
		return
	}
	var in models.Animal
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	existing.Name = in.Name
	existing.Species = in.Species
	existing.Tag = in.Tag
	if err := a.Store.UpdateAnimal(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (a *API) deleteAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, id); !ok {
		return
	}
	if err := a.Store.DeleteAnimal(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Readings ---

func (a *API) listReadings(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, id); !ok {
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	readings, err := a.Store.ListReadings(id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, readings)
}

// --- Alerts ---

func (a *API) listAlertsForAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, id); !ok {
		return
	}
	alerts, err := a.Store.ListAlertsForAnimal(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (a *API) listActiveAlerts(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	var alerts []models.Alert
	var err error
	if user.Role == models.RoleAdmin {
		alerts, err = a.Store.ListActiveAlerts()
	} else {
		alerts, err = a.Store.ListActiveAlertsByOwner(user.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (a *API) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	alert, err := a.Store.GetAlert(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "alert not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, alert.AnimalID); !ok {
		return
	}
	if err := a.Store.ResolveAlert(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Vaccinations ---

func (a *API) listVaccinations(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, id); !ok {
		return
	}
	vs, err := a.Store.ListVaccinations(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (a *API) createVaccination(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, ok := a.loadOwnedAnimal(w, r, id); !ok {
		return
	}
	var in models.Vaccination
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.ID = 0
	in.AnimalID = id
	if err := a.Store.CreateVaccination(&in); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}
