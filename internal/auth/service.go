// Package auth implements stateful, database-backed authentication:
// password hashing, session token generation, and the session lifecycle
// (login creates a DB row, logout deletes it, validation checks
// expiration). It knows nothing about HTTP — that's internal/middleware.
package auth

import (
	"errors"
	"time"

	"livestock/internal/models"
	"livestock/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrSessionInvalid     = errors.New("session is invalid or expired")
)

// DefaultSessionDuration is how long a session remains valid after login
// if the Service isn't configured with a different duration.
const DefaultSessionDuration = 24 * time.Hour

type Service struct {
	store           *storage.Store
	SessionDuration time.Duration
}

func NewService(store *storage.Store) *Service {
	return &Service{store: store, SessionDuration: DefaultSessionDuration}
}

// Register creates a new user account with a bcrypt-hashed password.
func (s *Service) Register(email, password string, role models.Role) (*models.User, error) {
	if _, err := s.store.GetUserByEmail(email); err == nil {
		return nil, ErrEmailTaken
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &models.User{Email: email, Password: hash, Role: role}
	if err := s.store.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login verifies credentials and creates a brand-new database session
// (a fresh random token every time, which also prevents session fixation:
// an attacker-supplied token from before authentication is never reused).
func (s *Service) Login(email, password string) (*models.User, *models.Session, error) {
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if !CheckPassword(user.Password, password) {
		return nil, nil, ErrInvalidCredentials
	}

	token, err := GenerateToken()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	session := &models.Session{
		UserID:         user.ID,
		SessionToken:   token,
		ExpiresAt:      now.Add(s.SessionDuration),
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	if err := s.store.CreateSession(session); err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

// Logout invalidates a session by deleting it from the database.
func (s *Service) Logout(token string) error {
	return s.store.DeleteSessionByToken(token)
}

// ValidateSession looks up the session token in the database, rejects it
// if missing/expired, and otherwise refreshes LastAccessedAt and returns
// the associated user. This is called on every authenticated request.
func (s *Service) ValidateSession(token string) (*models.User, *models.Session, error) {
	if token == "" {
		return nil, nil, ErrSessionInvalid
	}

	session, err := s.store.GetValidSession(token)
	if err != nil {
		return nil, nil, ErrSessionInvalid
	}

	user, err := s.store.GetUser(session.UserID)
	if err != nil {
		return nil, nil, ErrSessionInvalid
	}

	now := time.Now()
	_ = s.store.TouchSession(session.ID, now)
	session.LastAccessedAt = now

	return user, session, nil
}
