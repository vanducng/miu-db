package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SourceAuto = "auto"
	SourceFile = "file"
)

type StoreOptions struct {
	Source          string
	ConfigDir       string
	ConnectionsPath string
	CredentialsPath string
	SecretSources   []string
	KeyringService  string
	GopassPrefix    string
	SecretTimeout   time.Duration
}

type StoreInfo struct {
	Source          string   `json:"source"`
	ConfigDir       string   `json:"config_dir,omitempty"`
	ConnectionsPath string   `json:"connections_path"`
	CredentialsPath string   `json:"credentials_path,omitempty"`
	SecretSources   []string `json:"secret_sources"`
	KeyringService  string   `json:"keyring_service,omitempty"`
	GopassPrefix    string   `json:"gopass_prefix,omitempty"`
}

type AddOptions struct {
	SecretStore string
}

type Store struct {
	ConfigDir       string
	ConnectionsPath string
	CredentialsPath string
	Source          string
	SecretSources   []string
	KeyringService  string
	GopassPrefix    string
	resolvers       []SecretResolver
	secretTimeout   time.Duration
	root            Root
}

func DefaultConfigDir() string {
	if v := os.Getenv("MIUDB_CONFIG_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("MIU_DB_CONFIG_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "miu", "db")
}

func DefaultStoreOptions() StoreOptions {
	return StoreOptions{
		Source:        SourceAuto,
		ConfigDir:     DefaultConfigDir(),
		GopassPrefix:  "miudb",
		SecretTimeout: 5 * time.Second,
	}
}

func NewStore(configDir, credentialsPath string) (*Store, error) {
	opts := DefaultStoreOptions()
	opts.Source = SourceFile
	opts.ConfigDir = configDir
	opts.CredentialsPath = credentialsPath
	opts.SecretSources = []string{"file"}
	return NewStoreWithOptions(opts)
}

func NewStoreWithOptions(opts StoreOptions) (*Store, error) {
	return newStoreWithOptions(opts, false)
}

func NewWritableStore(opts StoreOptions) (*Store, error) {
	return newStoreWithOptions(opts, true)
}

func newStoreWithOptions(opts StoreOptions, allowMissing bool) (*Store, error) {
	opts = normalizeOptions(opts)
	connectionsPath, source, err := resolveConnectionsPath(opts)
	if err != nil {
		return nil, err
	}
	if opts.KeyringService == "" {
		opts.KeyringService = "miudb"
	}
	root, err := loadRoot(connectionsPath)
	if err != nil {
		if !allowMissing || !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		root = Root{Version: 1, Connections: []Connection{}}
	}
	resolvers, sources, credentialsPath, err := buildSecretResolvers(opts, source)
	if err != nil {
		return nil, err
	}
	if opts.CredentialsPath == "" {
		opts.CredentialsPath = credentialsPath
	}
	store := &Store{
		ConfigDir:       opts.ConfigDir,
		ConnectionsPath: connectionsPath,
		CredentialsPath: opts.CredentialsPath,
		Source:          source,
		SecretSources:   sources,
		KeyringService:  opts.KeyringService,
		GopassPrefix:    opts.GopassPrefix,
		resolvers:       resolvers,
		secretTimeout:   opts.SecretTimeout,
		root:            root,
	}
	return store, nil
}

func normalizeOptions(opts StoreOptions) StoreOptions {
	if opts.Source == "" {
		opts.Source = SourceAuto
	}
	opts.Source = strings.ToLower(opts.Source)
	if opts.ConfigDir == "" {
		opts.ConfigDir = DefaultConfigDir()
	}
	if opts.GopassPrefix == "" {
		opts.GopassPrefix = "miudb"
	}
	if opts.SecretTimeout <= 0 {
		opts.SecretTimeout = 5 * time.Second
	}
	return opts
}

func resolveConnectionsPath(opts StoreOptions) (string, string, error) {
	if opts.ConnectionsPath != "" {
		return expandHome(opts.ConnectionsPath), SourceFile, nil
	}
	switch opts.Source {
	case SourceFile:
		return filepath.Join(expandHome(opts.ConfigDir), "connections.json"), SourceFile, nil
	case SourceAuto:
		return filepath.Join(expandHome(opts.ConfigDir), "connections.json"), SourceFile, nil
	default:
		return "", "", fmt.Errorf("unknown connection source %q", opts.Source)
	}
}

func loadRoot(path string) (Root, error) {
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return Root{}, fmt.Errorf("read connections: %w", err)
	}
	var root Root
	if err := json.Unmarshal(data, &root); err == nil && len(root.Connections) > 0 {
		return root, nil
	}
	var list []Connection
	if err := json.Unmarshal(data, &list); err == nil {
		return Root{Version: 1, Connections: list}, nil
	}
	var payload struct {
		Version     int          `json:"version"`
		Connections []Connection `json:"connections"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Root{}, fmt.Errorf("parse connections: %w", err)
	}
	return Root{Version: payload.Version, Connections: payload.Connections}, nil
}

func (s *Store) Info() StoreInfo {
	return StoreInfo{
		Source:          s.Source,
		ConfigDir:       s.ConfigDir,
		ConnectionsPath: s.ConnectionsPath,
		CredentialsPath: s.CredentialsPath,
		SecretSources:   append([]string{}, s.SecretSources...),
		KeyringService:  s.KeyringService,
		GopassPrefix:    s.GopassPrefix,
	}
}

func (s *Store) Connections() []Connection {
	out := make([]Connection, len(s.root.Connections))
	copy(out, s.root.Connections)
	return out
}

func (s *Store) Find(name string) (Connection, bool) {
	for _, conn := range s.root.Connections {
		if conn.Name == name {
			resolved, err := s.Resolve(conn)
			if err == nil {
				conn = resolved
			}
			return conn, true
		}
	}
	return Connection{}, false
}

func (s *Store) FindResolved(name string) (Connection, bool, error) {
	for _, conn := range s.root.Connections {
		if conn.Name == name {
			resolved, err := s.Resolve(conn)
			if err != nil {
				return conn, true, err
			}
			return resolved, true, nil
		}
	}
	return Connection{}, false, nil
}

func (s *Store) Add(conn Connection, opts AddOptions) (Connection, error) {
	if strings.TrimSpace(conn.Name) == "" {
		return conn, fmt.Errorf("connection name is required")
	}
	if strings.TrimSpace(conn.DBType) == "" {
		return conn, fmt.Errorf("connection db_type is required")
	}
	if _, ok := s.FindRaw(conn.Name); ok {
		return conn, fmt.Errorf("connection %q already exists", conn.Name)
	}
	sanitized, err := s.persistSecrets(conn, opts)
	if err != nil {
		return conn, err
	}
	s.root.Connections = append(s.root.Connections, sanitized)
	if s.root.Version == 0 {
		s.root.Version = 1
	}
	if err := s.save(); err != nil {
		return conn, err
	}
	return sanitized, nil
}

func (s *Store) FindRaw(name string) (Connection, bool) {
	for _, conn := range s.root.Connections {
		if conn.Name == name {
			return conn, true
		}
	}
	return Connection{}, false
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.ConnectionsPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.ConnectionsPath, append(data, '\n'), 0o600)
}

type ImportOptions struct {
	DryRun bool
}

type ImportResult struct {
	Total       int      `json:"total"`
	Added       []string `json:"added"`
	Overwritten []string `json:"overwritten"`
	BackupPath  string   `json:"backup_path,omitempty"`
	DryRun      bool     `json:"dry_run"`
}

// LoadConnectionsFile parses a connections JSON file in Root, list, or payload form.
func LoadConnectionsFile(path string) ([]Connection, error) {
	root, err := loadRoot(path)
	if err != nil {
		return nil, err
	}
	return root.Connections, nil
}

// Import merges connections by name, overwriting same-named entries, and backs
// up the existing connections file before writing. Secrets are imported as-is.
func (s *Store) Import(conns []Connection, opts ImportOptions) (ImportResult, error) {
	res := ImportResult{Total: len(conns), Added: []string{}, Overwritten: []string{}, DryRun: opts.DryRun}
	if len(conns) == 0 {
		return res, fmt.Errorf("no connections to import")
	}
	for _, conn := range conns {
		if strings.TrimSpace(conn.Name) == "" {
			return res, fmt.Errorf("imported connection is missing a name")
		}
		if strings.TrimSpace(conn.DBType) == "" {
			return res, fmt.Errorf("imported connection %q is missing db_type", conn.Name)
		}
	}

	merged := make([]Connection, len(s.root.Connections))
	copy(merged, s.root.Connections)
	index := make(map[string]int, len(merged))
	for i, conn := range merged {
		index[conn.Name] = i
	}
	for _, conn := range conns {
		if i, ok := index[conn.Name]; ok {
			merged[i] = conn
			res.Overwritten = append(res.Overwritten, conn.Name)
		} else {
			index[conn.Name] = len(merged)
			merged = append(merged, conn)
			res.Added = append(res.Added, conn.Name)
		}
	}

	if opts.DryRun {
		return res, nil
	}

	if _, err := os.Stat(s.ConnectionsPath); err == nil {
		backup, err := backupFile(s.ConnectionsPath)
		if err != nil {
			return res, err
		}
		res.BackupPath = backup
	} else if !errors.Is(err, os.ErrNotExist) {
		return res, err
	}

	s.root.Connections = merged
	if s.root.Version == 0 {
		s.root.Version = 1
	}
	if err := s.save(); err != nil {
		return res, err
	}
	return res, nil
}

func backupFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	backup := fmt.Sprintf("%s.bak-%s", path, stamp)
	for i := 1; ; i++ {
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			break
		}
		backup = fmt.Sprintf("%s.bak-%s-%d", path, stamp, i)
	}
	if err := os.WriteFile(backup, data, 0o600); err != nil {
		return "", err
	}
	return backup, nil
}

func (s *Store) persistSecrets(conn Connection, opts AddOptions) (Connection, error) {
	store := strings.ToLower(strings.TrimSpace(opts.SecretStore))
	if store == "" {
		store = "keyring"
	}
	switch store {
	case "keyring":
		if conn.Endpoint.Password != "" {
			if err := StoreKeyringSecret(s.KeyringService, conn.Name, "db", conn.Endpoint.Password); err != nil {
				return conn, err
			}
			conn.Endpoint.Password = ""
			conn.Secrets = appendSecretRef(conn.Secrets, SecretRef{Target: "endpoint.password", Kind: "db", Provider: "keyring", Ref: s.KeyringService + ":" + credentialKey(conn.Name, "db")})
		}
		if conn.Tunnel != nil && conn.Tunnel.Password != "" {
			if err := StoreKeyringSecret(s.KeyringService, conn.Name, "ssh", conn.Tunnel.Password); err != nil {
				return conn, err
			}
			conn.Tunnel.Password = ""
			conn.Secrets = appendSecretRef(conn.Secrets, SecretRef{Target: "tunnel.password", Kind: "ssh", Provider: "keyring", Ref: s.KeyringService + ":" + credentialKey(conn.Name, "ssh")})
		}
	case "file":
		path := s.CredentialsPath
		if path == "" {
			path = defaultCredentialPath(s.Source, s.ConfigDir)
			s.CredentialsPath = path
		}
		if conn.Endpoint.Password != "" {
			if err := StoreCredentialFileSecret(path, conn.Name, "db", conn.Endpoint.Password); err != nil {
				return conn, err
			}
			conn.Endpoint.Password = ""
			conn.Secrets = appendSecretRef(conn.Secrets, SecretRef{Target: "endpoint.password", Kind: "db", Provider: "file", Ref: path})
		}
		if conn.Tunnel != nil && conn.Tunnel.Password != "" {
			if err := StoreCredentialFileSecret(path, conn.Name, "ssh", conn.Tunnel.Password); err != nil {
				return conn, err
			}
			conn.Tunnel.Password = ""
			conn.Secrets = appendSecretRef(conn.Secrets, SecretRef{Target: "tunnel.password", Kind: "ssh", Provider: "file", Ref: path})
		}
		if !hasSecretSource(s.SecretSources, "file") {
			s.SecretSources = append(s.SecretSources, "file")
		}
	case "inline":
	case "none":
		conn.Endpoint.Password = ""
		if conn.Tunnel != nil {
			conn.Tunnel.Password = ""
		}
	default:
		return conn, fmt.Errorf("unknown secret store %q", opts.SecretStore)
	}
	return conn, nil
}

func hasSecretSource(sources []string, name string) bool {
	for _, source := range sources {
		if strings.TrimPrefix(source, name+":") == source && source == name {
			return true
		}
		if strings.HasPrefix(source, name+":") {
			return true
		}
	}
	return false
}

func appendSecretRef(refs []SecretRef, ref SecretRef) []SecretRef {
	for i, existing := range refs {
		if existing.Target == ref.Target && existing.Kind == ref.Kind {
			refs[i] = ref
			return refs
		}
	}
	return append(refs, ref)
}

func (s *Store) Resolve(conn Connection) (Connection, error) {
	if err := resolveEndpointSecret(&conn, s.resolvers, s.secretTimeout); err != nil {
		return conn, err
	}
	if conn.Tunnel != nil {
		if conn.Tunnel.KeyPath == "" && conn.Tunnel.SSHKeyPath != "" {
			conn.Tunnel.KeyPath = conn.Tunnel.SSHKeyPath
		}
		if err := resolveTunnelSecret(&conn, s.resolvers, s.secretTimeout); err != nil {
			return conn, err
		}
	}
	return conn, nil
}

func resolveEndpointSecret(conn *Connection, resolvers []SecretResolver, timeout time.Duration) error {
	if conn.Endpoint.Password != "" {
		return nil
	}
	if conn.Endpoint.Kind == "file" {
		return nil
	}
	if !endpointPasswordRequired(*conn) {
		return nil
	}
	if conn.Endpoint.PasswordCommand != "" {
		secret, ok, err := resolveCommand(context.Background(), conn.Endpoint.PasswordCommand, timeout)
		if err != nil {
			return fmt.Errorf("resolve database password for %s: %w", conn.Name, err)
		}
		if ok {
			conn.Endpoint.Password = secret
			return nil
		}
	}
	value, ok, err := resolveSecret(*conn, "db", resolvers, timeout)
	if err != nil {
		return fmt.Errorf("resolve database password for %s: %w", conn.Name, err)
	}
	if ok {
		conn.Endpoint.Password = value
	}
	return nil
}

func endpointPasswordRequired(conn Connection) bool {
	if conn.Endpoint.PasswordCommand != "" || len(SecretRefsFor(conn, "db")) > 0 {
		return true
	}
	switch strings.ToLower(conn.DBType) {
	case "bigquery":
		return false
	case "snowflake":
		auth, _ := conn.Options["authenticator"].(string)
		return !strings.EqualFold(auth, "snowflake_jwt")
	}
	return conn.Endpoint.Username != ""
}

func resolveTunnelSecret(conn *Connection, resolvers []SecretResolver, timeout time.Duration) error {
	if conn.Tunnel == nil || conn.Tunnel.Password != "" {
		return nil
	}
	if !conn.Tunnel.Enabled {
		return nil
	}
	if conn.Tunnel.PasswordCommand != "" {
		secret, ok, err := resolveCommand(context.Background(), conn.Tunnel.PasswordCommand, timeout)
		if err != nil {
			return fmt.Errorf("resolve SSH password for %s: %w", conn.Name, err)
		}
		if ok {
			conn.Tunnel.Password = secret
			return nil
		}
	}
	value, ok, err := resolveSecret(*conn, "ssh", resolvers, timeout)
	if err != nil {
		return fmt.Errorf("resolve SSH password for %s: %w", conn.Name, err)
	}
	if ok {
		conn.Tunnel.Password = value
	}
	return nil
}

func resolveSecret(conn Connection, kind string, resolvers []SecretResolver, timeout time.Duration) (string, bool, error) {
	var firstErr error
	for _, ref := range SecretRefsFor(conn, kind) {
		value, ok, err := resolveSecretRef(conn, ref, timeout)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			return value, true, nil
		}
	}
	for _, resolver := range resolvers {
		value, ok, err := resolveWithTimeout(resolver, conn, kind, timeout)
		if err != nil {
			if errors.Is(err, ErrSecretNotFound) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			return value, true, nil
		}
	}
	if firstErr != nil {
		return "", false, firstErr
	}
	return "", false, nil
}

func resolveWithTimeout(resolver SecretResolver, conn Connection, kind string, timeout time.Duration) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	type result struct {
		value string
		ok    bool
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		value, ok, err := resolver.Resolve(ctx, conn, kind)
		ch <- result{value: value, ok: ok, err: err}
	}()
	select {
	case res := <-ch:
		return res.value, res.ok, res.err
	case <-ctx.Done():
		return "", false, ctx.Err()
	}
}

func credentialKey(connection, kind string) string {
	return connection + ":" + kind
}
