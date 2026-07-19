package auth

import "testing"

func TestHashPassword_VerifiesCorrectly(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("hash must not equal the plaintext password")
	}
	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("expected CheckPassword to accept the correct password")
	}
}

func TestCheckPassword_RejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected CheckPassword to reject an incorrect password")
	}
}

func TestGenerateToken_UniqueAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken returned error: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}
		if seen[token] {
			t.Fatalf("GenerateToken produced a duplicate: %s", token)
		}
		seen[token] = true
	}
}
