package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	currentTime := time.Now().UTC()

	claims := &jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(currentTime),
		ExpiresAt: jwt.NewNumericDate(currentTime.Add(expiresIn)),
		Subject:   userID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", errors.New("Failed to sign token")
	}

	return signedToken, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.UUID{}, errors.New("Invalid secret or token")
	}

	claimedToken, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, errors.New("Failed to get token's subject")
	}

	id, err := uuid.Parse(claimedToken)
	if err != nil {
		return uuid.UUID{}, errors.New("Could not parse the token")
	}

	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")
	if bearer == "" {
		return "", errors.New("Authorization header is missing")
	}

	return strings.Fields(bearer)[1], nil
}
