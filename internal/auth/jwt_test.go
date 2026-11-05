package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	testCases := []struct {
		name        string
		userID      uuid.UUID
		tokenSecret string
		expiresIn   time.Duration
	}{
		{"Create JWT", uuid.New(), "testSecret1", time.Hour},
		{"Wrong secret", uuid.New(), "testSecret2", time.Hour},
		{"Expired token", uuid.New(), "testSecret3", -time.Hour},
	}

	for testNumber, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			signedToken, err := MakeJWT(tt.userID, tt.tokenSecret, tt.expiresIn)
			if err != nil {
				t.Fatal(err)
			}

			switch testNumber {
			case 0:
				id, err := ValidateJWT(signedToken, tt.tokenSecret)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if tt.userID != id {
					t.Fatal("Validated jwt's id does not match user id")
				}
			case 1:
				_, err := ValidateJWT(signedToken, "wrongSecret")
				if err == nil {
					t.Fatal("Wrong secret returns an error")
				}
			case 2:
				_, err := ValidateJWT(signedToken, tt.tokenSecret)
				if err == nil {
					t.Fatal("Expired token throws an error")
				}
			}
		})
	}
}
