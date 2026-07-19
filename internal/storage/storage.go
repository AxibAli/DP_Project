package storage

import (
	"fmt"
	"log"

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

func (s *Store) AutoMigrate() error {
	return s.DB.AutoMigrate(&models.Animal{}, &models.Reading{}, &models.Alert{}, &models.Vaccination{})
}

// SeedIfEmpty inserts a handful of demo animals if the Animal table has no rows.
func (s *Store) SeedIfEmpty() error {
	var count int64
	if err := s.DB.Model(&models.Animal{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	demo := []models.Animal{
		{Name: "Bessie", Species: "cow", Tag: "Cow-01"},
		{Name: "Daisy", Species: "cow", Tag: "Cow-02"},
		{Name: "Wooly", Species: "sheep", Tag: "Sheep-01"},
		{Name: "Porky", Species: "pig", Tag: "Pig-01"},
		{Name: "Clucky", Species: "chicken", Tag: "Chicken-01"},
	}
	if err := s.DB.Create(&demo).Error; err != nil {
		return err
	}
	log.Printf("seeded %d demo animals", len(demo))
	return nil
}

// --- Animals ---

func (s *Store) ListAnimals() ([]models.Animal, error) {
	var animals []models.Animal
	err := s.DB.Order("id").Find(&animals).Error
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

func (s *Store) LatestReadingsByAnimal(animalIDs []uint) (map[uint]models.Reading, error) {
	result := make(map[uint]models.Reading)
	for _, id := range animalIDs {
		r, err := s.LatestReading(id)
		if err == nil {
			result[id] = *r
		}
	}
	return result, nil
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

func (s *Store) ListActiveAlerts() ([]models.Alert, error) {
	var alerts []models.Alert
	err := s.DB.Where("resolved = ?", false).Order("sequence_no desc").Find(&alerts).Error
	return alerts, err
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
