package repository

import (
	"strings"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
)

func testSpec() entities.ContainerSpec {
	return entities.ContainerSpec{
		DatabaseID:  "db-a",
		Image:       "postgres:17",
		Password:    "secret",
		Port:        21000,
		HostDataDir: "/var/lib/mbr-node/mounts/db-a",
		MemoryBytes: 256 * 1024 * 1024,
		NanoCPUs:    250_000_000,
		TLSCertPath: "/etc/mbr-node/tenant.crt",
		TLSKeyPath:  "/etc/mbr-node/tenant.key",
	}
}

func TestBuildContainerConfigMountsTLSCertAndKeyReadOnly(t *testing.T) {
	spec := testSpec()
	_, hostConfig := buildContainerConfig(spec)

	wantCertBind := spec.TLSCertPath + ":" + tenantTLSCertMountPath + ":ro"
	wantKeyBind := spec.TLSKeyPath + ":" + tenantTLSKeyMountPath + ":ro"
	wantDataBind := spec.HostDataDir + ":/var/lib/postgresql/data"

	for _, want := range []string{wantDataBind, wantCertBind, wantKeyBind} {
		found := false
		for _, b := range hostConfig.Binds {
			if b == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("hostConfig.Binds = %v, want to contain %q", hostConfig.Binds, want)
		}
	}
}

func TestBuildContainerConfigEnablesSSL(t *testing.T) {
	spec := testSpec()
	config, _ := buildContainerConfig(spec)

	want := []string{
		"ssl=on",
		"ssl_cert_file=" + tenantTLSCertMountPath,
		"ssl_key_file=" + tenantTLSKeyMountPath,
		"hba_file=/var/lib/postgresql/data/" + tenantHBAConfName,
	}
	for _, w := range want {
		found := false
		for _, arg := range config.Cmd {
			if arg == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("config.Cmd = %v, want to contain %q", config.Cmd, w)
		}
	}
}

func TestBuildContainerConfigPreservesExistingBehavior(t *testing.T) {
	spec := testSpec()
	config, hostConfig := buildContainerConfig(spec)

	if config.Image != spec.Image {
		t.Fatalf("config.Image = %q, want %q", config.Image, spec.Image)
	}
	wantEnv := []string{
		"POSTGRES_PASSWORD=" + spec.Password,
		"POSTGRES_DB=postgres",
		"PGDATA=/var/lib/postgresql/data/pgdata",
	}
	for _, w := range wantEnv {
		found := false
		for _, e := range config.Env {
			if e == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("config.Env = %v, want to contain %q", config.Env, w)
		}
	}
	if hostConfig.Resources.Memory != spec.MemoryBytes || hostConfig.Resources.NanoCPUs != spec.NanoCPUs {
		t.Fatalf("hostConfig.Resources = %+v, want memory %d and %d nanoCPUs", hostConfig.Resources, spec.MemoryBytes, spec.NanoCPUs)
	}
	if len(hostConfig.PortBindings[postgresContainerPort]) != 1 || hostConfig.PortBindings[postgresContainerPort][0].HostPort != "21000" {
		t.Fatalf("hostConfig.PortBindings = %+v, want host port 21000", hostConfig.PortBindings)
	}
}

func TestTenantHBAConfRequiresSSLOnly(t *testing.T) {
	conf := tenantHBAConf()

	if !strings.Contains(conf, "hostssl") {
		t.Fatalf("tenantHBAConf() = %q, want a hostssl rule", conf)
	}
	for _, line := range strings.Split(conf, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "host" {
			t.Fatalf("tenantHBAConf() contains a plaintext host rule: %q", line)
		}
	}
}
