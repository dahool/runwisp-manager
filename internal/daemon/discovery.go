package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/runwisp/runwisp-manager/internal/config"
	"github.com/runwisp/runwisp-manager/internal/runwispbin"
)

type Capabilities struct {
	Transport      Transport `json:"transport"`
	DaemonVersion  string    `json:"daemonVersion"`
	BinaryVersion  string    `json:"binaryVersion"`
	VersionOK      bool      `json:"versionOK"`
	VersionMatch   bool      `json:"versionMatch"`
	ConfigPath     string    `json:"configPath"`
	ConfigSource   string    `json:"configSource"`
	ConfigReadable bool      `json:"configReadable"`
	ConfigWritable bool      `json:"configWritable"`
	CanEdit        bool      `json:"canEdit"`
	ReadOnlyReason string    `json:"readOnlyReason,omitempty"`
	DaemonPID      int       `json:"daemonPID,omitempty"`
	Warnings       []string  `json:"warnings,omitempty"`
}

type Discovery struct {
	Client        Client
	Config        config.Config
	BinaryVersion string
	Daemon        DaemonInfo
	Capabilities  Capabilities
	LastError     error
}

func (d *Discovery) Refresh(ctx context.Context) error {
	d.Capabilities = Capabilities{Transport: d.Client.Transport(), BinaryVersion: d.BinaryVersion}
	var info DaemonInfo
	if d.Client.Transport() == TransportUnix {
		identity, err := d.Client.Identity(ctx)
		if err != nil {
			d.LastError = err
			return err
		}
		info = DaemonInfo{Version: identity.Version, Fingerprint: identity.Fingerprint}
		d.Capabilities.ConfigPath, d.Capabilities.ConfigSource, d.Capabilities.DaemonPID = identity.ConfigPath, identity.ConfigSource, identity.PID
	} else {
		var err error
		info, err = d.Client.Info(ctx)
		if err != nil {
			d.LastError = err
			return err
		}
		d.Capabilities.ConfigPath = d.Config.RunWispConfig
		if d.Capabilities.ConfigPath == "" {
			d.LastError = fmt.Errorf("RWM_RUNWISP_CONFIG is required for HTTP transport")
			return d.LastError
		}
	}
	d.Capabilities.DaemonVersion = info.Version
	d.Daemon = info
	policy := runwispbin.CheckVersion(info.Version)
	d.Capabilities.VersionOK = policy.Supported
	if policy.Warning != "" {
		d.Capabilities.Warnings = append(d.Capabilities.Warnings, policy.Warning)
		if d.Config.StrictVersion {
			d.LastError = fmt.Errorf("%s", policy.Warning)
			return d.LastError
		}
	}
	d.Capabilities.VersionMatch = d.BinaryVersion == "" || info.Version == d.BinaryVersion
	if !d.Capabilities.VersionMatch {
		warning := fmt.Sprintf("RunWisp daemon version %s differs from binary version %s", info.Version, d.BinaryVersion)
		d.Capabilities.Warnings = append(d.Capabilities.Warnings, warning)
		if d.Config.StrictVersion {
			d.LastError = fmt.Errorf("%s", warning)
			return d.LastError
		}
	}
	if d.Capabilities.ConfigPath != "" {
		d.Capabilities.ConfigReadable = readable(d.Capabilities.ConfigPath)
		dir := filepath.Dir(d.Capabilities.ConfigPath)
		d.Capabilities.ConfigWritable = writable(dir)
		if !d.Capabilities.ConfigReadable {
			d.Capabilities.ReadOnlyReason = "config file is not readable"
		} else if !d.Capabilities.ConfigWritable {
			d.Capabilities.ReadOnlyReason = "config dir not writable"
		}
	}
	if d.Capabilities.ConfigSource == "app" {
		d.Capabilities.ReadOnlyReason = "config supplied by SDK app"
	}
	d.Capabilities.CanEdit = d.Capabilities.ConfigSource != "app" && d.Capabilities.ConfigReadable && d.Capabilities.ConfigWritable && d.Config.EditEnabled
	if !d.Capabilities.CanEdit && d.Capabilities.ReadOnlyReason == "" {
		d.Capabilities.ReadOnlyReason = "editing disabled"
	}
	d.LastError = nil
	return nil
}

func (d *Discovery) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = d.Refresh(ctx)
			}
		}
	}()
}
func readable(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }
func writable(dir string) bool {
	file, err := os.CreateTemp(dir, ".runwisp-manager-probe-*")
	if err != nil {
		return false
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return false
	}
	return os.Remove(name) == nil
}
