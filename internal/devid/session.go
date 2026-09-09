package devid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Session holds in-flight enrollment state after a successful start.
type Session struct {
	ID        string
	Nonce     []byte
	Request   SigningRequest
	SubjectCN string
	Created   time.Time
}

// SessionStore is an in-memory enroll session map.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewSessionStore creates a session store with the given TTL.
func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// Put stores a session and returns its ID.
func (s *SessionStore) Put(nonce []byte, sr SigningRequest, subjectCN string) (string, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked()
	s.sessions[id] = &Session{
		ID:        id,
		Nonce:     append([]byte{}, nonce...),
		Request:   sr,
		SubjectCN: subjectCN,
		Created:   time.Now(),
	}
	return id, nil
}

// Take removes and returns a session by ID.
func (s *SessionStore) Take(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("unknown or expired session")
	}
	delete(s.sessions, id)
	return sess, nil
}

func (s *SessionStore) purgeLocked() {
	now := time.Now()
	for id, sess := range s.sessions {
		if now.Sub(sess.Created) > s.ttl {
			delete(s.sessions, id)
		}
	}
}
