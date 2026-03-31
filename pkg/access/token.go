// Package access provides optional token-based socket authentication.
// Tokens are generated with crypto/rand and validated against SHA-256
// HMAC hashes stored in a JSON file.
package access

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/valksor/kvelmo/pkg/paths"
)

// Role defines the access level for a token.
type Role string

const (
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Token represents a stored authentication token.
type Token struct {
	ID        string     `json:"id"`
	Hash      string     `json:"hash"`
	Role      Role       `json:"role"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Store manages tokens in a JSON file, using SHA-256 HMAC for hashing.
type Store struct {
	path string
}

// storeData is the on-disk format for the token store.
type storeData struct {
	HMACKey string  `json:"hmac_key"`
	Tokens  []Token `json:"tokens"`
}

// New creates a Store that persists tokens at path.
// If path is empty, defaults to <BaseDir>/tokens.json.
func New(path string) *Store {
	if path == "" {
		path = filepath.Join(paths.BaseDir(), "tokens.json")
	}

	return &Store{path: path}
}

// Create generates a new random token, stores its HMAC hash, and
// returns the plaintext token string.
func (s *Store) Create(role Role, label string, expiry *time.Time) (string, error) {
	sd, err := s.load()
	if err != nil {
		return "", err
	}

	// Generate random token (32 bytes, hex encoded).
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	plaintext := hex.EncodeToString(tokenBytes)

	// Generate token ID (8 bytes, hex encoded).
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	hash, err := s.hmacHash(sd.HMACKey, plaintext)
	if err != nil {
		return "", fmt.Errorf("hash token: %w", err)
	}

	token := Token{
		ID:        hex.EncodeToString(idBytes),
		Hash:      hash,
		Role:      role,
		Label:     label,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiry,
	}

	sd.Tokens = append(sd.Tokens, token)

	if err := s.save(sd); err != nil {
		return "", err
	}

	return plaintext, nil
}

// Validate checks a plaintext token against all stored tokens.
// Returns the matching Token or an error if no match is found.
// Expired tokens are rejected.
func (s *Store) Validate(plaintext string) (*Token, error) {
	sd, err := s.load()
	if err != nil {
		return nil, err
	}

	hash, err := s.hmacHash(sd.HMACKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("hash token: %w", err)
	}

	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("decode computed hash: %w", err)
	}

	for i := range sd.Tokens {
		storedBytes, decErr := hex.DecodeString(sd.Tokens[i].Hash)
		if decErr != nil {
			continue // Skip malformed stored hashes
		}
		if hmac.Equal(storedBytes, hashBytes) {
			if sd.Tokens[i].ExpiresAt != nil && sd.Tokens[i].ExpiresAt.Before(time.Now()) {
				return nil, errors.New("token expired")
			}

			result := sd.Tokens[i]

			return &result, nil
		}
	}

	return nil, errors.New("invalid token")
}

// Revoke removes a token by ID.
func (s *Store) Revoke(id string) error {
	sd, err := s.load()
	if err != nil {
		return err
	}

	found := false
	tokens := make([]Token, 0, len(sd.Tokens))

	for _, t := range sd.Tokens {
		if t.ID == id {
			found = true

			continue
		}

		tokens = append(tokens, t)
	}

	if !found {
		return fmt.Errorf("token %q not found", id)
	}

	sd.Tokens = tokens

	return s.save(sd)
}

// List returns all tokens with the Hash field cleared for security.
func (s *Store) List() ([]Token, error) {
	sd, err := s.load()
	if err != nil {
		return nil, err
	}

	result := make([]Token, len(sd.Tokens))
	for i, t := range sd.Tokens {
		t.Hash = ""
		result[i] = t
	}

	return result, nil
}

func (s *Store) hmacHash(key, plaintext string) (string, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("decode hmac key: %w", err)
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(plaintext))

	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *Store) load() (*storeData, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		// Initialize with a new random HMAC key.
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			return nil, fmt.Errorf("generate hmac key: %w", err)
		}

		return &storeData{
			HMACKey: hex.EncodeToString(keyBytes),
			Tokens:  nil,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read token store: %w", err)
	}

	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("parse token store: %w", err)
	}

	return &sd, nil
}

func (s *Store) save(sd *storeData) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("ensure token dir: %w", err)
	}

	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token store: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write token store: %w", err)
	}

	return nil
}
