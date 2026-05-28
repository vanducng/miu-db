package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Root struct {
	Version     int          `json:"version"`
	Connections []Connection `json:"connections"`
}

type Connection struct {
	Name          string            `json:"name"`
	DBType        string            `json:"db_type"`
	Source        string            `json:"source,omitempty"`
	ConnectionURL string            `json:"connection_url,omitempty"`
	FolderPath    string            `json:"folder_path,omitempty"`
	ExtraOptions  map[string]string `json:"extra_options,omitempty"`
	Options       map[string]any    `json:"options,omitempty"`
	Endpoint      Endpoint          `json:"endpoint"`
	Tunnel        *Tunnel           `json:"tunnel,omitempty"`
}

type Endpoint struct {
	Kind            string `json:"kind"`
	Host            string `json:"host,omitempty"`
	Port            string `json:"port,omitempty"`
	Database        string `json:"database,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	PasswordCommand string `json:"password_command,omitempty"`
	Path            string `json:"path,omitempty"`
}

type Tunnel struct {
	Enabled         bool   `json:"enabled"`
	Source          string `json:"source,omitempty"`
	ConfigAlias     string `json:"config_alias,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            string `json:"port,omitempty"`
	Username        string `json:"username,omitempty"`
	AuthType        string `json:"auth_type,omitempty"`
	Password        string `json:"password,omitempty"`
	PasswordCommand string `json:"password_command,omitempty"`
	KeyPath         string `json:"key_path,omitempty"`
}

type Store struct {
	ConfigDir       string
	CredentialsPath string
	root            Root
	credentials     map[string]string
}

func DefaultConfigDir() string {
	if v := os.Getenv("MIU_DB_CONFIG_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "miu", "db")
}

func NewStore(configDir, credentialsPath string) (*Store, error) {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}
	if credentialsPath == "" {
		credentialsPath = filepath.Join(configDir, "credentials-export.json")
	}
	store := &Store{ConfigDir: configDir, CredentialsPath: credentialsPath}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) load() error {
	path := filepath.Join(s.ConfigDir, "connections.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read connections: %w", err)
	}
	if err := json.Unmarshal(data, &s.root); err != nil {
		return fmt.Errorf("parse connections: %w", err)
	}
	credentials, err := LoadCredentialExport(s.CredentialsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.credentials = credentials
	s.applyCredentials()
	return nil
}

func (s *Store) Connections() []Connection {
	out := make([]Connection, len(s.root.Connections))
	copy(out, s.root.Connections)
	return out
}

func (s *Store) Find(name string) (Connection, bool) {
	for _, conn := range s.root.Connections {
		if conn.Name == name {
			return conn, true
		}
	}
	return Connection{}, false
}

func (s *Store) applyCredentials() {
	for i := range s.root.Connections {
		conn := &s.root.Connections[i]
		if conn.Endpoint.Password == "" {
			conn.Endpoint.Password = s.credentials[credentialKey(conn.Name, "db")]
		}
		if conn.Tunnel != nil && conn.Tunnel.Password == "" {
			conn.Tunnel.Password = s.credentials[credentialKey(conn.Name, "ssh")]
		}
	}
}

func credentialKey(connection, kind string) string {
	return connection + ":" + kind
}

func RedactedConnection(conn Connection) map[string]any {
	endpoint := map[string]any{
		"kind":     conn.Endpoint.Kind,
		"host":     conn.Endpoint.Host,
		"port":     conn.Endpoint.Port,
		"database": conn.Endpoint.Database,
		"username": conn.Endpoint.Username,
		"path":     conn.Endpoint.Path,
	}
	if conn.Endpoint.Password != "" {
		endpoint["has_password"] = true
	}
	tunnel := map[string]any{"enabled": false}
	if conn.Tunnel != nil {
		tunnel = map[string]any{
			"enabled":      conn.Tunnel.Enabled,
			"source":       conn.Tunnel.Source,
			"config_alias": conn.Tunnel.ConfigAlias,
			"host":         conn.Tunnel.Host,
			"port":         conn.Tunnel.Port,
			"username":     conn.Tunnel.Username,
			"auth_type":    conn.Tunnel.AuthType,
			"key_path":     conn.Tunnel.KeyPath,
		}
		if conn.Tunnel.Password != "" {
			tunnel["has_password"] = true
		}
	}
	return map[string]any{
		"name":        conn.Name,
		"db_type":     conn.DBType,
		"folder_path": conn.FolderPath,
		"endpoint":    endpoint,
		"tunnel":      tunnel,
		"options":     RedactOptions(conn.Options),
	}
}

func RedactOptions(options map[string]any) map[string]any {
	if options == nil {
		return nil
	}
	out := make(map[string]any, len(options))
	for key, value := range options {
		if isSecretKey(key) {
			if value != nil && fmt.Sprint(value) != "" {
				out[key] = map[string]any{"redacted": true}
			}
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = RedactOptions(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, token := range []string{"password", "secret", "token", "private_key", "client_secret", "tls_key"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
