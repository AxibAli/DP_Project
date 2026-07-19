// Package api exposes the REST/JSON CRUD surface over storage. It has no
// knowledge of sensors or rules — only how to read/write via the Store and
// serialize JSON.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"livestock/internal/models"
	"livestock/internal/storage"
)

type API struct {
	Store *storage.Store
}

func New(store *storage.Store) *API {
	return &API{Store: store}
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()

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

// --- Animals ---

func (a *API) listAnimals(w http.ResponseWriter, r *http.Request) {
	animals, err := a.Store.ListAnimals()
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
	in.ID = 0
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

func (a *API) getAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	animal, err := a.Store.GetAnimal(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "animal not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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
	existing, err := a.Store.GetAnimal(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "animal not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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
	alerts, err := a.Store.ListAlertsForAnimal(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (a *API) listActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := a.Store.ListActiveAlerts()
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
