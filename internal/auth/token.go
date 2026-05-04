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

	"github.com/google/uuid"
)

var errInvalidToken = errors.New("invalid token")

type tokenClaims struct {
	Subject  string `json:"sub"`
	Username string `json:"username"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

func signJWT(secret []byte, user *User, ttl time.Duration) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := tokenClaims{
		Subject:  user.ID.String(),
		Username: user.Username,
		IssuedAt: now.Unix(),
		Expires:  now.Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	payload := encodedHeader + "." + encodedClaims
	signature := signPayload(secret, payload)
	return payload + "." + signature, nil
}

func parseJWT(secret []byte, token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}

	payload := parts[0] + "." + parts[1]
	expected := signPayload(secret, payload)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errInvalidToken
	}

	rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return nil, errInvalidToken
	}
	if claims.Expires < time.Now().Unix() {
		return nil, errors.New("token expired")
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("%w: subject", errInvalidToken)
	}
	return &claims, nil
}

func signPayload(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
