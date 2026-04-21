package setup

// DesktopSession is a read-only bootstrap payload for desktop shells.
type DesktopSession struct {
	Ready               bool   `json:"ready"`
	Reason              string `json:"reason,omitempty"`
	WorkspaceID         string `json:"workspace_id,omitempty"`
	BackendURL          string `json:"backend_url,omitempty"`
	AuthType            string `json:"auth_type,omitempty"`
	AccessToken         string `json:"access_token,omitempty"`
	CredentialBackend   string `json:"credential_backend,omitempty"`
	SecureStorageReady  bool   `json:"secure_storage_ready"`
	DesktopSessionReady bool   `json:"desktop_session_ready"`
}

// LoadDesktopSession returns a fail-closed desktop bootstrap payload.
func LoadDesktopSession() DesktopSession {
	snapshot, _ := loadCredentialSnapshot()
	status := collectStatusFromSnapshot(snapshot)
	session := DesktopSession{
		WorkspaceID:         status.WorkspaceID,
		BackendURL:          status.BackendURL,
		AuthType:            status.AuthType,
		CredentialBackend:   status.CredentialBackend,
		SecureStorageReady:  status.SecureStorageReady,
		DesktopSessionReady: status.DesktopSessionReady,
	}

	switch {
	case !status.Configured:
		session.Reason = "worker_not_configured"
		return session
	case !status.SecureStorageReady:
		session.Reason = "secure_storage_unavailable"
		return session
	case !status.AuthValid:
		session.Reason = "auth_invalid"
		return session
	case status.AuthType != "jwt":
		session.Reason = "jwt_required_for_desktop"
		return session
	case status.WorkspaceID == "":
		session.Reason = "workspace_missing"
		return session
	case status.BackendURL == "":
		session.Reason = "backend_missing"
		return session
	}

	token := authTokenFromCredentials(snapshot.creds)
	if token == "" {
		session.Reason = "token_unavailable"
		return session
	}

	session.Ready = true
	session.AccessToken = token
	return session
}
