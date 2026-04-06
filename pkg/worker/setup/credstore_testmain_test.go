package setup

// NOTE: Do NOT add a TestMain with keyring.MockInit() here.
// The go-keyring mock provider is not goroutine-safe.
// Each keychain test must call keyring.MockInit() individually
// and must NOT use t.Parallel().
