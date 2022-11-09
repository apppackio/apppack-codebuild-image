package aws

import "testing"

func TestECRTokenDecode(t *testing.T) {
	encoded := "QVdTOnBhc3N3b3Jk"
	username, password, err := decodeECRToken(encoded)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	expected := "AWS"
	if username != expected {
		t.Errorf("expected %s, got %s", expected, username)
	}
	expected = "password"
	if password != expected {
		t.Errorf("expected %s, got %s", expected, password)
	}
}

func TestECRTokenDecodeInvalid(t *testing.T) {
	encoded := "QVdTcGFzc3dvcmQ="
	_, _, err := decodeECRToken(encoded)
	if err == nil {
		t.Errorf("expected an error, got nil")
	}
}

func TestECRTokenDecodeInvalidBase64(t *testing.T) {
	encoded := "not-a-valid-token"
	_, _, err := decodeECRToken(encoded)
	if err == nil {
		t.Errorf("expected an error, got nil")
	}
}
