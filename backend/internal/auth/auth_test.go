package auth

import "testing"

func TestTokenRoundTrip(t *testing.T) {
	manager := NewManager("test-secret")
	token, err := manager.Token(7, "analyst@example.com", "analyst")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 7 || claims.Role != "analyst" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if CheckPassword(hash, "correct horse") != nil || CheckPassword(hash, "wrong password") == nil {
		t.Fatal("password check failed")
	}
}
