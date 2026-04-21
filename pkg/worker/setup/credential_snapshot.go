package setup

import (
	"encoding/json"
	"os"
	"time"
)

type credentialPayload struct {
	data    []byte
	backend string
	secure  bool
}

type credentialSnapshot struct {
	backend string
	secure  bool
	creds   *rawCredentials
}

var loadCredentialPayloadFunc = defaultLoadCredentialPayload

func loadCredentialPayload() (credentialPayload, error) {
	return loadCredentialPayloadFunc()
}

func loadCredentialPayloadFromStore(
	store CredentialStore,
	backend string,
	secure bool,
) (credentialPayload, error) {
	raw, err := store.Load(workerCredentialService)
	if err != nil || raw == "" {
		return credentialPayload{}, os.ErrNotExist
	}
	return credentialPayload{
		data:    []byte(raw),
		backend: backend,
		secure:  secure,
	}, nil
}

func loadPlaintextCredentialPayload() (credentialPayload, error) {
	data, err := os.ReadFile(DefaultCredentialsPath())
	if err != nil {
		return credentialPayload{backend: "none"}, err
	}
	if len(data) == 0 {
		return credentialPayload{backend: "none"}, os.ErrNotExist
	}

	return credentialPayload{
		data:    data,
		backend: "plaintext_file",
		secure:  false,
	}, nil
}

func defaultLoadCredentialPayload() (credentialPayload, error) {
	if payload, err := loadCredentialPayloadFromStore(newKeychainStore(), "keychain", true); err == nil {
		return payload, nil
	}

	if payload, err := loadCredentialPayloadFromStore(
		newEncryptedFileStore(defaultCredentialDir()),
		"encrypted_file",
		true,
	); err == nil {
		return payload, nil
	}

	return loadPlaintextCredentialPayload()
}

func loadCredentialSnapshot() (credentialSnapshot, error) {
	payload, err := loadCredentialPayload()
	if err != nil {
		return credentialSnapshot{
			backend: payload.backend,
			secure:  payload.secure,
		}, err
	}

	var creds rawCredentials
	if err := json.Unmarshal(payload.data, &creds); err != nil {
		return credentialSnapshot{
			backend: payload.backend,
			secure:  payload.secure,
		}, err
	}

	return credentialSnapshot{
		backend: payload.backend,
		secure:  payload.secure,
		creds:   &creds,
	}, nil
}

func authStateFromCredentials(creds *rawCredentials) (authValid bool, authType string) {
	if creds == nil {
		return false, "none"
	}

	if creds.APIKey != "" {
		return true, "api_key"
	}

	if creds.AccessToken != "" {
		if creds.ExpiresAt == "" {
			return true, "jwt"
		}
		expiry, err := time.Parse(time.RFC3339, creds.ExpiresAt)
		if err != nil {
			return true, "jwt"
		}
		return time.Until(expiry) > 5*time.Minute, "jwt"
	}

	return false, "none"
}

func authTokenFromCredentials(creds *rawCredentials) string {
	if creds == nil {
		return ""
	}
	if creds.AuthType == "api_key" && creds.APIKey != "" {
		return creds.APIKey
	}
	if creds.AccessToken != "" {
		return creds.AccessToken
	}
	return ""
}
