package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/99designs/keyring"
)

var ErrSecretNotFound = errors.New("secret not found")

type SecretResolver interface {
	Source() string
	Resolve(ctx context.Context, conn Connection, kind string) (string, bool, error)
}

func SecretRefsFor(conn Connection, kind string) []SecretRef {
	out := []SecretRef{}
	for _, ref := range conn.Secrets {
		if ref.Kind != "" && ref.Kind != kind {
			continue
		}
		switch kind {
		case "db":
			if ref.Target == "endpoint.password" || ref.Target == "password" {
				out = append(out, ref)
			}
		case "ssh":
			if ref.Target == "tunnel.password" || ref.Target == "ssh_password" {
				out = append(out, ref)
			}
		}
	}
	return out
}

func SensitiveTargets(conn Connection) []string {
	targets := []string{}
	if conn.Endpoint.Password != "" || len(SecretRefsFor(conn, "db")) > 0 {
		targets = append(targets, "endpoint.password")
	}
	if conn.Tunnel != nil && (conn.Tunnel.Password != "" || len(SecretRefsFor(conn, "ssh")) > 0) {
		targets = append(targets, "tunnel.password")
	}
	for key, value := range conn.Options {
		if strings.HasPrefix(key, "__") {
			continue
		}
		if isSecretKey(key) && fmt.Sprint(value) != "" {
			targets = append(targets, "options."+key)
		}
	}
	return targets
}

func buildSecretResolvers(opts StoreOptions, source string) ([]SecretResolver, []string, string, error) {
	names := normalizeSecretSources(opts.SecretSources, source)
	resolvers := []SecretResolver{}
	active := []string{}
	credentialsPath := opts.CredentialsPath
	for _, name := range names {
		switch name {
		case "file":
			explicitPath := opts.CredentialsPath != ""
			path := opts.CredentialsPath
			if path == "" {
				path = defaultCredentialPath(source, opts.ConfigDir)
			}
			if path == "" {
				continue
			}
			if credentialsPath == "" {
				credentialsPath = path
			}
			items, err := LoadCredentialFile(expandHome(path))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) && !explicitPath {
					legacyPath := legacyCredentialExportPath(opts.ConfigDir)
					legacyItems, legacyErr := LoadCredentialFile(expandHome(legacyPath))
					if legacyErr == nil {
						resolvers = append(resolvers, newStaticSecretResolver("file", legacyItems))
						active = append(active, "file")
						credentialsPath = legacyPath
						continue
					}
					if !errors.Is(legacyErr, os.ErrNotExist) {
						return nil, nil, "", legacyErr
					}
				}
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, nil, "", err
			}
			resolvers = append(resolvers, newStaticSecretResolver("file", items))
			active = append(active, "file")
		case "keyring":
			resolver, err := newKeyringSecretResolver(opts.KeyringService)
			if err != nil {
				if len(opts.SecretSources) == 0 {
					continue
				}
				return nil, nil, "", err
			}
			resolvers = append(resolvers, resolver)
			active = append(active, "keyring:"+opts.KeyringService)
		case "gopass":
			resolvers = append(resolvers, &gopassSecretResolver{prefix: opts.GopassPrefix})
			active = append(active, "gopass:"+opts.GopassPrefix)
		case "none":
		case "":
		default:
			return nil, nil, "", fmt.Errorf("unknown secret source %q", name)
		}
	}
	return resolvers, active, credentialsPath, nil
}

func normalizeSecretSources(sources []string, connectionSource string) []string {
	if len(sources) == 0 {
		return []string{"file", "keyring", "gopass"}
	}
	out := []string{}
	for _, source := range sources {
		for _, part := range strings.Split(source, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func defaultCredentialPath(source, configDir string) string {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}
	return filepath.Join(expandHome(configDir), "credentials.json")
}

func legacyCredentialExportPath(configDir string) string {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}
	return filepath.Join(expandHome(configDir), "credentials-export.json")
}

type keyringSecretResolver struct {
	service string
	ring    keyring.Keyring
}

func newKeyringSecretResolver(service string) (*keyringSecretResolver, error) {
	if service == "" {
		service = "miudb"
	}
	return &keyringSecretResolver{service: service}, nil
}

// keyringConfig builds the keyring backend config. The FileDir fallback matters
// for CGO-disabled release builds (goreleaser sets CGO_ENABLED=0), where the OS
// keychain backend is absent and keyring would otherwise error with "no directory
// provided for file keyring". On CGO builds the OS keychain stays preferred.
func keyringConfig(service string) keyring.Config {
	if service == "" {
		service = "miudb"
	}
	fileDir := filepath.Join(expandHome(DefaultConfigDir()), "keyring")
	return keyring.Config{
		ServiceName:              service,
		KeychainName:             "login",
		KeychainTrustApplication: true,
		FileDir:                  fileDir,
		FilePasswordFunc:         keyring.FixedStringPrompt("miudb"),
	}
}

func openKeyring(service string) (keyring.Keyring, error) {
	cfg := keyringConfig(service)
	_ = os.MkdirAll(cfg.FileDir, 0o700)
	ring, err := keyring.Open(cfg)
	if err != nil {
		return nil, err
	}
	return ring, nil
}

func (r *keyringSecretResolver) keyring() (keyring.Keyring, error) {
	if r.ring != nil {
		return r.ring, nil
	}
	ring, err := openKeyring(r.service)
	if err != nil {
		return nil, err
	}
	r.ring = ring
	return ring, nil
}

func (r *keyringSecretResolver) Source() string { return "keyring:" + r.service }

func (r *keyringSecretResolver) Resolve(ctx context.Context, conn Connection, kind string) (string, bool, error) {
	ring, err := r.keyring()
	if err != nil {
		return "", false, err
	}
	item, err := ring.Get(credentialKey(conn.Name, kind))
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(item.Data), true, nil
}

func StoreKeyringSecret(service, connection, kind, value string) error {
	if value == "" {
		return nil
	}
	if service == "" {
		service = "miudb"
	}
	ring, err := openKeyring(service)
	if err != nil {
		return err
	}
	return ring.Set(keyring.Item{
		Key:         credentialKey(connection, kind),
		Data:        []byte(value),
		Label:       fmt.Sprintf("%s %s password", connection, kind),
		Description: "Stored by miudb",
	})
}

func DeleteKeyringSecret(service, connection, kind string) error {
	if service == "" {
		service = "miudb"
	}
	ring, err := openKeyring(service)
	if err != nil {
		return err
	}
	err = ring.Remove(credentialKey(connection, kind))
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	return err
}

type gopassSecretResolver struct {
	prefix string
}

func (r *gopassSecretResolver) Source() string { return "gopass:" + r.prefix }

func (r *gopassSecretResolver) Resolve(ctx context.Context, conn Connection, kind string) (string, bool, error) {
	paths := []string{}
	if r.prefix != "" {
		paths = append(paths,
			filepath.ToSlash(filepath.Join(r.prefix, conn.Name, kind)),
			filepath.ToSlash(filepath.Join(r.prefix, credentialKey(conn.Name, kind))),
		)
	}
	paths = append(paths, credentialKey(conn.Name, kind))
	for _, path := range paths {
		out, err := exec.CommandContext(ctx, "gopass", "show", "--password", path).Output()
		if err == nil {
			value := strings.TrimRight(string(out), "\r\n")
			return value, true, nil
		}
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
	}
	return "", false, nil
}

func resolveSecretRef(conn Connection, ref SecretRef, timeout time.Duration) (string, bool, error) {
	switch strings.ToLower(ref.Provider) {
	case "keyring":
		service, key := splitServiceRef(ref.Ref, "miudb")
		ring, err := openKeyring(service)
		if err != nil {
			return "", false, err
		}
		item, err := ring.Get(key)
		if err != nil {
			if errors.Is(err, keyring.ErrKeyNotFound) {
				return "", false, nil
			}
			return "", false, err
		}
		return string(item.Data), true, nil
	case "gopass":
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "gopass", "show", "--password", ref.Ref).Output()
		if err != nil {
			if ctx.Err() != nil {
				return "", false, ctx.Err()
			}
			return "", false, nil
		}
		return strings.TrimRight(string(out), "\r\n"), true, nil
	case "command":
		return resolveCommand(context.Background(), ref.Ref, timeout)
	case "env":
		value := os.Getenv(ref.Ref)
		return value, value != "", nil
	case "file":
		items, err := LoadCredentialFile(ref.Ref)
		if err != nil {
			return "", false, err
		}
		kind := ref.Kind
		if kind == "" {
			kind = "db"
		}
		if value, ok := items[credentialKey(conn.Name, kind)]; ok {
			return value, true, nil
		}
		return "", false, nil
	default:
		return "", false, fmt.Errorf("unknown secret provider %q", ref.Provider)
	}
}

func splitServiceRef(ref, fallbackService string) (string, string) {
	service, key := fallbackService, ref
	if strings.Contains(ref, ":") {
		parts := strings.SplitN(ref, ":", 2)
		service, key = parts[0], parts[1]
	}
	return service, key
}

func resolveCommand(ctx context.Context, command string, timeout time.Duration) (string, bool, error) {
	if strings.TrimSpace(command) == "" {
		return "", false, nil
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "sh", "-c", command).Output()
	if err != nil {
		return "", false, err
	}
	return strings.TrimRight(string(out), "\r\n"), true, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
