package storage

import (
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"livestock/internal/models"
)

type Store struct {
	DB *gorm.DB
}

func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	return &Store{DB: db}, nil
}

// AutoMigrate creates/updates tables. Users and Sessions are migrated before
// Animal so the OwnerID/UserID foreign keys can be created.
func (s *Store) AutoMigrate() error {
	return s.DB.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.Animal{},
		&models.Reading{},
		&models.Alert{},
		&models.Vaccination{},
	)
}

// SeedIfEmpty inserts a demo admin, a demo user, and a handful of demo
// animals owned by the demo user if the Animal table has no rows.
// Passwords are supplied already-hashed by the caller (main), keeping
// storage free of hashing/auth concerns.
func (s *Store) SeedIfEmpty(adminEmail, adminPasswordHash, userEmail, userPasswordHash string) error {
	var animalCount int64
	if err := s.DB.Model(&models.Animal{}).Count(&animalCount).Error; err != nil {
		return err
	}
	if animalCount > 0 {
		return nil
	}

	admin, err := s.getOrCreateUser(adminEmail, adminPasswordHash, models.RoleAdmin)
	if err != nil {
		return err
	}
	demoUser, err := s.getOrCreateUser(userEmail, userPasswordHash, models.RoleUser)
	if err != nil {
		return err
	}

	demo := []models.Animal{
		{Name: "Bessie", Species: "cow", Tag: "Cow-01", OwnerID: demoUser.ID},
		{Name: "Daisy", Species: "cow", Tag: "Cow-02", OwnerID: demoUser.ID},
		{Name: "Wooly", Species: "sheep", Tag: "Sheep-01", OwnerID: demoUser.ID},
		{Name: "Porky", Species: "pig", Tag: "Pig-01", OwnerID: admin.ID},
		{Name: "Clucky", Species: "chicken", Tag: "Chicken-01", OwnerID: admin.ID},
	}
	if err := s.DB.Create(&demo).Error; err != nil {
		return err
	}
	log.Printf("seeded %d demo animals for admin=%s user=%s", len(demo), adminEmail, userEmail)
	return nil
}

func (s *Store) getOrCreateUser(email, passwordHash string, role models.Role) (*models.User, error) {
	var u models.User
	err := s.DB.Where("email = ?", email).First(&u).Error
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	u = models.User{Email: email, Password: passwordHash, Role: role}
	if err := s.DB.Create(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// --- Users ---

func (s *Store) CreateUser(u *models.User) error {
	return s.DB.Create(u).Error
}

func (s *Store) GetUserByEmail(email string) (*models.User, error) {
	var u models.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUser(id uint) (*models.User, error) {
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListUsers() ([]models.User, error) {
	var users []models.User
	err := s.DB.Order("id").Find(&users).Error
	return users, err
}

func (s *Store) UpdateUserRole(id uint, role models.Role) error {
	return s.DB.Model(&models.User{}).Where("id = ?", id).Update("role", role).Error
}

func (s *Store) DeleteUser(id uint) error {
	return s.DB.Delete(&models.User{}, id).Error
}

// --- Sessions ---

func (s *Store) CreateSession(sess *models.Session) error {
	return s.DB.Create(sess).Error
}

// GetValidSession returns the session for the given token only if it
// exists and has not expired. Expired-but-present sessions are treated as
// not found so callers uniformly reject them.
func (s *Store) GetValidSession(token string) (*models.Session, error) {
	var sess models.Session
	err := s.DB.Where("session_token = ? AND expires_at > ?", token, time.Now()).First(&sess).Error
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) TouchSession(id uint, lastAccessedAt time.Time) error {
	return s.DB.Model(&models.Session{}).Where("id = ?", id).Update("last_accessed_at", lastAccessedAt).Error
}

func (s *Store) DeleteSessionByToken(token string) error {
	return s.DB.Where("session_token = ?", token).Delete(&models.Session{}).Error
}

func (s *Store) DeleteExpiredSessions() error {
	return s.DB.Where("expires_at <= ?", time.Now()).Delete(&models.Session{}).Error
}

// --- Animals ---

// ListAnimals returns every animal (admin view).
func (s *Store) ListAnimals() ([]models.Animal, error) {
	var animals []models.Animal
	err := s.DB.Order("id").Find(&animals).Error
	return animals, err
}

// ListAnimalsByOwner returns only animals owned by the given user.
func (s *Store) ListAnimalsByOwner(ownerID uint) ([]models.Animal, error) {
	var animals []models.Animal
	err := s.DB.Where("owner_id = ?", ownerID).Order("id").Find(&animals).Error
	return animals, err
}

func (s *Store) CreateAnimal(a *models.Animal) error {
	return s.DB.Create(a).Error
}

func (s *Store) GetAnimal(id uint) (*models.Animal, error) {
	var a models.Animal
	if err := s.DB.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) UpdateAnimal(a *models.Animal) error {
	return s.DB.Save(a).Error
}

func (s *Store) DeleteAnimal(id uint) error {
	return s.DB.Delete(&models.Animal{}, id).Error
}

func (s *Store) LatestReading(animalID uint) (*models.Reading, error) {
	var r models.Reading
	err := s.DB.Where("animal_id = ?", animalID).Order("sequence_no desc").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// --- Readings ---

func (s *Store) CreateReading(r *models.Reading) error {
	return s.DB.Create(r).Error
}

func (s *Store) ListReadings(animalID uint, limit, offset int) ([]models.Reading, error) {
	var readings []models.Reading
	q := s.DB.Where("animal_id = ?", animalID).Order("sequence_no desc")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	err := q.Find(&readings).Error
	return readings, err
}

// --- Alerts ---

func (s *Store) CreateAlert(a *models.Alert) error {
	return s.DB.Create(a).Error
}

func (s *Store) ListAlertsForAnimal(animalID uint) ([]models.Alert, error) {
	var alerts []models.Alert
	err := s.DB.Where("animal_id = ?", animalID).Order("sequence_no desc").Find(&alerts).Error
	return alerts, err
}

// ListActiveAlerts returns every unresolved alert (admin view).
func (s *Store) ListActiveAlerts() ([]models.Alert, error) {
	var alerts []models.Alert
	err := s.DB.Where("resolved = ?", false).Order("sequence_no desc").Find(&alerts).Error
	return alerts, err
}

// ListActiveAlertsByOwner returns unresolved alerts scoped to animals owned
// by the given user, via a join on animals.owner_id.
func (s *Store) ListActiveAlertsByOwner(ownerID uint) ([]models.Alert, error) {
	var alerts []models.Alert
	err := s.DB.
		Joins("JOIN animals ON animals.id = alerts.animal_id").
		Where("alerts.resolved = ? AND animals.owner_id = ?", false, ownerID).
		Order("alerts.sequence_no desc").
		Find(&alerts).Error
	return alerts, err
}

func (s *Store) GetAlert(id uint) (*models.Alert, error) {
	var a models.Alert
	if err := s.DB.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ResolveAlert(id uint) error {
	return s.DB.Model(&models.Alert{}).Where("id = ?", id).Update("resolved", true).Error
}

// --- Vaccinations ---

func (s *Store) ListVaccinations(animalID uint) ([]models.Vaccination, error) {
	var vs []models.Vaccination
	err := s.DB.Where("animal_id = ?", animalID).Order("date_given desc").Find(&vs).Error
	return vs, err
}

func (s *Store) CreateVaccination(v *models.Vaccination) error {
	return s.DB.Create(v).Error
}
