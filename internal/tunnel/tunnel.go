package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/vanducng/miu-db/internal/config"
)

type Forward struct {
	Host     string
	Port     string
	listener net.Listener
	client   *ssh.Client
	done     chan struct{}
	once     sync.Once
}

func Open(ctx context.Context, cfg config.Tunnel, targetHost, targetPort string) (*Forward, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	clientConfig, err := sshClientConfig(resolved)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(resolved.Host, defaultString(resolved.Port, "22"))
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	rawConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	client := ssh.NewClient(rawConn, chans, reqs)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	f := &Forward{Host: "127.0.0.1", Port: port, listener: listener, client: client, done: make(chan struct{})}
	go f.accept(ctx, net.JoinHostPort(targetHost, targetPort))
	return f, nil
}

func (f *Forward) Close() error {
	var err error
	f.once.Do(func() {
		close(f.done)
		err = f.listener.Close()
		if closeErr := f.client.Close(); err == nil {
			err = closeErr
		}
	})
	return err
}

func (f *Forward) accept(ctx context.Context, remoteAddr string) {
	for {
		local, err := f.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			remote, err := f.client.Dial("tcp", remoteAddr)
			if err != nil {
				_ = local.Close()
				return
			}
			defer local.Close()
			defer remote.Close()
			go func() {
				select {
				case <-ctx.Done():
					_ = local.Close()
					_ = remote.Close()
				case <-f.done:
					_ = local.Close()
					_ = remote.Close()
				}
			}()
			go io.Copy(remote, local)
			_, _ = io.Copy(local, remote)
		}()
	}
}

func sshClientConfig(cfg config.Tunnel) (*ssh.ClientConfig, error) {
	auths := []ssh.AuthMethod{}
	if cfg.KeyPath != "" {
		key, err := os.ReadFile(expandPath(cfg.KeyPath))
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("ssh tunnel requires key_path or password")
	}
	return &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func resolveConfig(cfg config.Tunnel) (config.Tunnel, error) {
	if cfg.Source != "config" || cfg.ConfigAlias == "" {
		return cfg, nil
	}
	sshCfg, err := parseSSHConfig(expandPath("~/.ssh/config"), cfg.ConfigAlias)
	if err != nil {
		return cfg, err
	}
	cfg.Host = firstNonEmpty(cfg.Host, sshCfg["hostname"], cfg.ConfigAlias)
	cfg.Username = firstNonEmpty(cfg.Username, sshCfg["user"])
	cfg.Port = firstNonEmpty(cfg.Port, sshCfg["port"], "22")
	cfg.KeyPath = firstNonEmpty(cfg.KeyPath, sshCfg["identityfile"])
	return cfg, nil
}

func parseSSHConfig(path, alias string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	active := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")
		if key == "host" {
			active = false
			for _, pattern := range fields[1:] {
				if pattern == alias {
					active = true
					break
				}
			}
			continue
		}
		if active {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("ssh config alias %q not found", alias)
	}
	return result, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" || value == "0" {
		return fallback
	}
	if _, err := strconv.Atoi(value); err != nil {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
