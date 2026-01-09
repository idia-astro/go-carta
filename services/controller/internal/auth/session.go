package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"
)

// TODO: in production, load this from config or env, not hard-coded.
var sessionSecret = []byte("CHANGE-ME-TO-A-RANDOM-SECRET")

// GenerateSessionToken creates a signed token for a username with an expiry time.
func GenerateSessionToken(username string, expiry time.Time) (string, error) {
	payload := fmt.Sprintf("%s|%d", username, expiry.Unix())
	log.Printf("Generating session token with payload: %s", payload)
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)

	token := payload + "|" + base64.RawURLEncoding.EncodeToString(sig)
	return base64.RawURLEncoding.EncodeToString([]byte(token)), nil
}

// VerifySessionToken checks the token signature and expiry and returns the username.
func VerifySessionToken(token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("bad token encoding")
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", fmt.Errorf("bad token format")
	}
	username := parts[0]
	expiryStr := parts[1]
	sigB64 := parts[2]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", fmt.Errorf("bad sig encoding")
	}

	payload := username + "|" + expiryStr

	log.Printf("Verifying session token with payload: %s", payload)
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	expected := mac.Sum(nil)

	if !hmac.Equal(sig, expected) {
		return "", fmt.Errorf("invalid signature")
	}

	var expiryUnix int64
	_, err = fmt.Sscanf(expiryStr, "%d", &expiryUnix)
	if err != nil {
		return "", fmt.Errorf("bad expiry")
	}
	if time.Now().Unix() > expiryUnix {
		return "", fmt.Errorf("session expired")
	}

	return username, nil
}
