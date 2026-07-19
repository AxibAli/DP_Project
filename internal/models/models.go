package models

import "time"

type Animal struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Species   string    `json:"species"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

type Reading struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AnimalID    uint      `gorm:"index" json:"animal_id"`
	SequenceNo  uint64    `gorm:"index" json:"sequence_no"`
	Temperature float64   `json:"temperature"`
	HeartRate   int       `json:"heart_rate"`
	Activity    int       `json:"activity"`
	CreatedAt   time.Time `json:"created_at"`
}

type Alert struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AnimalID   uint      `gorm:"index" json:"animal_id"`
	ReadingID  uint      `json:"reading_id"`
	SequenceNo uint64    `json:"sequence_no"`
	Reason     string    `json:"reason"`
	Severity   string    `json:"severity"`
	CreatedAt  time.Time `json:"created_at"`
	Resolved   bool      `gorm:"index" json:"resolved"`
}

type Vaccination struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	AnimalID  uint       `gorm:"index" json:"animal_id"`
	Name      string     `json:"name"`
	DateGiven time.Time  `json:"date_given"`
	NextDue   *time.Time `json:"next_due"`
}
