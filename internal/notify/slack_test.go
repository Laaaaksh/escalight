package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func sign(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":" + body))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySlackSignature_Valid(t *testing.T) {
	secret := "shh"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := "command=/ack&text=abc123"
	sig := sign(secret, ts, body)

	if !VerifySlackSignature(secret, ts, body, sig) {
		t.Error("expected valid signature to verify")
	}
}

func TestVerifySlackSignature_WrongSecret(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := "command=/ack"
	sig := sign("shh", ts, body)

	if VerifySlackSignature("different-secret", ts, body, sig) {
		t.Error("expected signature verification to fail with wrong secret")
	}
}

func TestVerifySlackSignature_TamperedBody(t *testing.T) {
	secret := "shh"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := sign(secret, ts, "command=/ack&text=abc123")

	if VerifySlackSignature(secret, ts, "command=/ack&text=evil", sig) {
		t.Error("expected signature verification to fail for a tampered body")
	}
}

func TestVerifySlackSignature_ExpiredTimestamp(t *testing.T) {
	secret := "shh"
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	body := "command=/ack"
	sig := sign(secret, old, body)

	if VerifySlackSignature(secret, old, body, sig) {
		t.Error("expected a request older than 5 minutes to be rejected (replay protection)")
	}
}
