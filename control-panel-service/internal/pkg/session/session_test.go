package session

import "testing"

func TestGenerateTokenIsRandomAndNonEmpty(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("GenerateToken returned an empty token")
	}
	if a == b {
		t.Fatal("two generated tokens were identical")
	}
}

func TestHashTokenIsDeterministicAndDistinct(t *testing.T) {
	first := HashToken("token-a")
	second := HashToken("token-a")
	if first != second {
		t.Fatal("HashToken is not deterministic")
	}
	if HashToken("token-a") == HashToken("token-b") {
		t.Fatal("HashToken produced the same hash for different tokens")
	}
}
