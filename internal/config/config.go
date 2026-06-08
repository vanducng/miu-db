package config

import (
	"fmt"
	"strings"
)

type Root struct {
	Version     int          `json:"version"`
	Connections []Connection `json:"connections"`
}

type Connection struct {
	Group         string            `json:"group,omitempty"` // company/org folder; first field for easy human review
	Name          string            `json:"name"`
	DBType        string            `json:"db_type"`
	Source        string            `json:"source,omitempty"`
	ConnectionURL string            `json:"connection_url,omitempty"`
	ExtraOptions  map[string]string `json:"extra_options,omitempty"`
	Options       map[string]any    `json:"options,omitempty"`
	Endpoint      Endpoint          `json:"endpoint"`
	Tunnel        *Tunnel           `json:"tunnel,omitempty"`
	Secrets       []SecretRef       `json:"secrets,omitempty"`
	LogSQL        *bool             `json:"log_sql,omitempty"`
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
	SSHKeyPath      string `json:"ssh_key_path,omitempty"`
}

type SecretRef struct {
	Target   string `json:"target"`
	Kind     string `json:"kind,omitempty"`
	Provider string `json:"provider"`
	Ref      string `json:"ref"`
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
	if len(SecretRefsFor(conn, "db")) > 0 {
		endpoint["has_password"] = true
		endpoint["secret_ref"] = true
	}
	tunnel := map[string]any{"enabled": false}
	if conn.Tunnel != nil {
		keyPath := conn.Tunnel.KeyPath
		if keyPath == "" {
			keyPath = conn.Tunnel.SSHKeyPath
		}
		tunnel = map[string]any{
			"enabled":      conn.Tunnel.Enabled,
			"source":       conn.Tunnel.Source,
			"config_alias": conn.Tunnel.ConfigAlias,
			"host":         conn.Tunnel.Host,
			"port":         conn.Tunnel.Port,
			"username":     conn.Tunnel.Username,
			"auth_type":    conn.Tunnel.AuthType,
			"key_path":     keyPath,
		}
		if conn.Tunnel.Password != "" {
			tunnel["has_password"] = true
		}
		if len(SecretRefsFor(conn, "ssh")) > 0 {
			tunnel["has_password"] = true
			tunnel["secret_ref"] = true
		}
	}
	return map[string]any{
		"name":     conn.Name,
		"db_type":  conn.DBType,
		"group":    conn.Group,
		"endpoint": endpoint,
		"tunnel":   tunnel,
		"options":  RedactOptions(conn.Options),
	}
}

func RedactOptions(options map[string]any) map[string]any {
	if options == nil {
		return nil
	}
	out := make(map[string]any, len(options))
	for key, value := range options {
		// Transient runtime keys (__ prefix) must never appear in any output.
		if strings.HasPrefix(key, "__") {
			continue
		}
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
