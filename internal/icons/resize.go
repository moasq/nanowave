package icons

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// Resize uses macOS sips to resize a PNG to the given pixel dimensions.
func Resize(src, dst string, pixels int) error {
	// Strip xattrs that can cause sips to reject the file (e.g. com.apple.provenance)
	_ = exec.Command("xattr", "-c", src).Run()
	cmd := exec.Command("sips", "-z", fmt.Sprintf("%d", pixels), fmt.Sprintf("%d", pixels), src, "--out", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[asc][icon] sips FAILED for %dx%d → %s: %s: %v", pixels, pixels, filepath.Base(dst), string(out), err)
		return fmt.Errorf("sips failed: %s: %w", string(out), err)
	}
	// Verify the output file exists and has content
	info, statErr := os.Stat(dst)
	if statErr != nil || info.Size() == 0 {
		log.Printf("[asc][icon] sips WARNING: output file %s missing or empty after resize", filepath.Base(dst))
	}
	return nil
}

// CopyFile copies src to dst.
func CopyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
