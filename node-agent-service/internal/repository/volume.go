package repository

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VolumeManager creates and destroys the fixed-size ext4-backed storage
// each tenant container mounts at /var/lib/postgresql/data. The volume is
// a loop-mounted ext4 image file rather than a Docker volume, so its size
// is enforced by the filesystem instead of merely measured.
type VolumeManager interface {
	// Create provisions (or, if it already exists, returns) the mount
	// for id sized to sizeGB GiB and returns the host path to bind into
	// the container.
	Create(id string, sizeGB int) (hostPath string, err error)
	// Remove unmounts and deletes the volume for id. It is a no-op if
	// the volume does not exist.
	Remove(id string) error
}

// loopVolumeManager implements VolumeManager with a sparse ext4 image
// file per tenant, mounted with the kernel's loop driver.
type loopVolumeManager struct {
	dataDir string
}

// NewLoopVolumeManager roots tenant volumes under dataDir, in
// "images/<id>.img" (the backing file) and "mounts/<id>" (the mountpoint).
func NewLoopVolumeManager(dataDir string) VolumeManager {
	return &loopVolumeManager{dataDir: dataDir}
}

func (m *loopVolumeManager) imagePath(id string) string {
	return filepath.Join(m.dataDir, "images", id+".img")
}

func (m *loopVolumeManager) mountPath(id string) string {
	return filepath.Join(m.dataDir, "mounts", id)
}

func (m *loopVolumeManager) Create(id string, sizeGB int) (string, error) {
	mountPath := m.mountPath(id)

	mounted, err := isMounted(mountPath)
	if err != nil {
		return "", fmt.Errorf("check mount for %s: %w", id, err)
	}
	if mounted {
		return mountPath, nil
	}

	imagePath := m.imagePath(id)
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o700); err != nil {
		return "", fmt.Errorf("create image dir: %w", err)
	}
	if err := os.MkdirAll(mountPath, 0o700); err != nil {
		return "", fmt.Errorf("create mount dir: %w", err)
	}

	if _, err := os.Stat(imagePath); err != nil {
		if err := createExt4Image(imagePath, sizeGB); err != nil {
			_ = os.Remove(imagePath)
			return "", err
		}
	}

	if err := runCommand("mount", "-o", "loop", imagePath, mountPath); err != nil {
		_ = os.Remove(imagePath)
		return "", fmt.Errorf("mount %s at %s: %w", imagePath, mountPath, err)
	}

	return mountPath, nil
}

func (m *loopVolumeManager) Remove(id string) error {
	mountPath := m.mountPath(id)

	mounted, err := isMounted(mountPath)
	if err != nil {
		return fmt.Errorf("check mount for %s: %w", id, err)
	}
	if mounted {
		if err := runCommand("umount", mountPath); err != nil {
			return fmt.Errorf("unmount %s: %w", mountPath, err)
		}
	}
	if err := os.RemoveAll(mountPath); err != nil {
		return fmt.Errorf("remove mount dir: %w", err)
	}
	if err := os.Remove(m.imagePath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
}

func createExt4Image(imagePath string, sizeGB int) error {
	f, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}
	size := int64(sizeGB) * 1024 * 1024 * 1024
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return fmt.Errorf("size image file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close image file: %w", err)
	}
	if err := runCommand("mkfs.ext4", "-F", "-q", imagePath); err != nil {
		return fmt.Errorf("format image: %w", err)
	}
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, out)
	}
	return nil
}

// isMounted reports whether path appears as a mountpoint in
// /proc/mounts.
func isMounted(path string) (bool, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == path {
			return true, nil
		}
	}
	return false, scanner.Err()
}
