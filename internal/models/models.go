package models

import "time"

type Role string

const (
	RoleAdmin Role = "ADMIN"
	RoleUser  Role = "USER"
)

// User is an authenticated account. Password holds a bcrypt hash, never
// the plaintext password.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password  string    `gorm:"column:password;size:255;not null" json:"-"`
	Role      Role      `gorm:"size:16;not null;index" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Session is a server-side session record. The cookie sent to the browser
// carries only the opaque SessionToken; all authorization state (user,
// expiry) lives here in the database.
type Session struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	User           User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	SessionToken   string    `gorm:"uniqueIndex;size:128;not null" json:"-"`
	ExpiresAt      time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

type Animal struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Species   string    `json:"species"`
	Tag       string    `json:"tag"` // e.g. "Cow-17"
	OwnerID   uint      `gorm:"not null;index" json:"owner_id"`
	Owner     User      `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type Reading struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AnimalID    uint      `gorm:"index" json:"animal_id"`
	SequenceNo  uint64    `gorm:"index" json:"sequence_no"` // monotonically increasing, assigned by ingest worker
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
	Reason     string    `json:"reason"`   // e.g. "High temperature + elevated heart rate"
	Severity   string    `json:"severity"` // "warning" | "critical"
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
