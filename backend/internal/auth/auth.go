package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Expiry int64  `json:"exp"`
}

type Manager struct{ secret []byte }

func NewManager(secret string) *Manager { return &Manager{secret: []byte(secret)} }

func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (manager *Manager) Sign(claims Claims) (string, error) {
	if len(manager.secret) == 0 {
		return "", errors.New("JWT secret is not configured")
	}
	header := encode(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := encode(claims)
	unsigned := header + "." + payload
	signature := hmac.New(sha256.New, manager.secret)
	_, _ = signature.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature.Sum(nil)), nil
}

func (manager *Manager) Verify(token string) (Claims, error) {
	var claims Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(manager.secret) == 0 {
		return claims, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, manager.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return claims, errors.New("invalid token signature")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(decoded, &claims) != nil || claims.Expiry <= time.Now().Unix() {
		return claims, errors.New("expired token")
	}
	return claims, nil
}

func (manager *Manager) Token(userID int64, email, role string) (string, error) {
	return manager.Sign(Claims{UserID: userID, Email: email, Role: role, Expiry: time.Now().Add(12 * time.Hour).Unix()})
}
func encode(value any) string {
	payload, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(payload)
}
func Bearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("authorization header must use Bearer token")
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix)), nil
}
