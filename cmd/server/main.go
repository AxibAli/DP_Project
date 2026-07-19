package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"livestock/internal/api"
	"livestock/internal/auth"
	"livestock/internal/dashboard"
	"livestock/internal/models"
	"livestock/internal/rules"
	"livestock/internal/sensors"
	"livestock/internal/storage"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dsn := getenv("DB_DSN", "sqlserver://sa:YourPassword@localhost:1433?database=livestock")
	port := getenv("PORT", "8080")
	sensorCount, err := strconv.Atoi(getenv("SENSOR_COUNT", "5"))
	if err != nil || sensorCount <= 0 {
		sensorCount = 5
	}
	// Secure cookies require HTTPS. Default to false so local http://
	// development works out of the box; set COOKIE_SECURE=true whenever
	// this is deployed behind TLS.
	secureCookies := getenv("COOKIE_SECURE", "false") == "true"

	adminEmail := getenv("ADMIN_EMAIL", "admin@livestock.local")
	adminPassword := getenv("ADMIN_PASSWORD", "AdminPass123!")
	demoUserEmail := getenv("DEMO_USER_EMAIL", "user@livestock.local")
	demoUserPassword := getenv("DEMO_USER_PASSWORD", "UserPass123!")

	store, err := storage.Open(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := store.AutoMigrate(); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	adminHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}
	userHash, err := auth.HashPassword(demoUserPassword)
	if err != nil {
		log.Fatalf("failed to hash demo user password: %v", err)
	}
	if err := store.SeedIfEmpty(adminEmail, adminHash, demoUserEmail, userHash); err != nil {
		log.Fatalf("failed to seed database: %v", err)
	}

	animals, err := store.ListAnimals()
	if err != nil {
		log.Fatalf("failed to load animals: %v", err)
	}
	if len(animals) > sensorCount {
		animals = animals[:sensorCount]
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	readings := sensors.StartAll(ctx, animals, 64)
	animalByID := make(map[uint]models.Animal, len(animals))
	for _, a := range animals {
		animalByID[a.ID] = a
	}

	var seq atomic.Uint64
	go runIngestWorker(ctx, store, readings, animalByID, &seq)

	authSvc := auth.NewService(store)
	go expireSessionsPeriodically(ctx, store)

	apiHandler := api.New(store, authSvc, secureCookies)
	dashboardHandler, err := dashboard.New(store, authSvc)
	if err != nil {
		log.Fatalf("failed to build dashboard templates: %v", err)
	}

	router := chi.NewRouter()
	router.Mount("/api", apiHandler.Routes())
	router.Mount("/", dashboardHandler.Routes())

	srv := &http.Server{Addr: ":" + port, Handler: router}

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

// expireSessionsPeriodically garbage-collects expired session rows so the
// sessions table doesn't grow unbounded. Rejection of expired sessions on
// use (GetValidSession) does not depend on this running; it's just hygiene.
func expireSessionsPeriodically(ctx context.Context, store *storage.Store) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := store.DeleteExpiredSessions(); err != nil {
				log.Printf("failed to clean up expired sessions: %v", err)
			}
		}
	}
}

// runIngestWorker consumes readings from the shared sensor channel, assigns
// a strictly increasing sequence number, persists each reading, evaluates it
// against the rules engine, and persists an Alert if abnormal. A bad reading
// or DB hiccup is logged and skipped rather than crashing the process.
func runIngestWorker(
	ctx context.Context,
	store *storage.Store,
	readings <-chan models.Reading,
	animalByID map[uint]models.Animal,
	seq *atomic.Uint64,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case reading, ok := <-readings:
			if !ok {
				return
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("ingest worker recovered from panic: %v", r)
					}
				}()

				reading.SequenceNo = seq.Add(1)

				if err := store.CreateReading(&reading); err != nil {
					log.Printf("failed to persist reading for animal %d: %v", reading.AnimalID, err)
					return
				}

				animal, ok := animalByID[reading.AnimalID]
				if !ok {
					log.Printf("unknown animal %d for reading, skipping rule evaluation", reading.AnimalID)
					return
				}

				eval := rules.Evaluate(animal.Species, reading)
				if !eval.Abnormal {
					return
				}

				alert := models.Alert{
					AnimalID:   reading.AnimalID,
					ReadingID:  reading.ID,
					SequenceNo: reading.SequenceNo,
					Reason:     eval.Reason,
					Severity:   eval.Severity,
					Resolved:   false,
				}
				if err := store.CreateAlert(&alert); err != nil {
					log.Printf("failed to persist alert for animal %d: %v", reading.AnimalID, err)
				}
			}()
		}
	}
}
