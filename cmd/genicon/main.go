package main

import (
	"image"
	"log"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"

	"ytmusic/internal/win32"
)

func main() {
	sizes := []int{16, 32, 48, 64, 128, 256}
	images := make([]image.Image, 0, len(sizes))
	for _, sz := range sizes {
		images = append(images, win32.RenderAppIconImage(sz))
	}

	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		log.Fatalf("create multi-resolution icon: %v", err)
	}

	outDir := filepath.Join("cmd", "ytmusic")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	icoPath := filepath.Join(outDir, "app.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		log.Fatalf("create %s: %v", icoPath, err)
	}
	if err := icon.SaveICO(icoFile); err != nil {
		_ = icoFile.Close()
		log.Fatalf("save %s: %v", icoPath, err)
	}
	_ = icoFile.Close()

	rs := winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(1), icon); err != nil {
		log.Fatalf("set icon resource: %v", err)
	}

	vi := version.Info{
		FileVersion:    [4]uint16{1, 0, 0, 0},
		ProductVersion: [4]uint16{1, 0, 0, 0},
	}
	vi.Set(0, version.ProductName, "ytmusic")
	vi.Set(0, version.FileDescription, "ytmusic — Minimalistic YouTube Music Player")
	vi.Set(0, version.OriginalFilename, "ytmusic.exe")
	rs.SetVersionInfo(vi)

	rs.SetManifest(winres.AppManifest{
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
	})

	sysoPath := filepath.Join(outDir, "rsrc_windows_amd64.syso")
	sysoFile, err := os.Create(sysoPath)
	if err != nil {
		log.Fatalf("create %s: %v", sysoPath, err)
	}
	if err := rs.WriteObject(sysoFile, winres.ArchAMD64); err != nil {
		_ = sysoFile.Close()
		log.Fatalf("write %s: %v", sysoPath, err)
	}
	_ = sysoFile.Close()
}
