package config

import "github.com/99designs/keyring"

// GetOAuthKeyringForTest returns the current override (nil = use OS keyring).
// Test seam: lets other packages' tests inspect the active keyring backend; not for production use.
func GetOAuthKeyringForTest() keyring.Keyring { return oauthKeyring }

// SetOAuthKeyringForTest replaces the keyring used by Store/Load/Delete.
// Test seam: lets other packages' tests inject a file-backend keyring; not for production use.
func SetOAuthKeyringForTest(k keyring.Keyring) { oauthKeyring = k }
