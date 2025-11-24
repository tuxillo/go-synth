package pkg

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dsynth/config"
)

// CRCDatabase manages build state tracking
type CRCDatabase struct {
	path    string
	entries map[string]*CRCEntry
	mu      sync.RWMutex
	dirty   bool
}

// CRCEntry represents a single package entry in the CRC database
type CRCEntry struct {
	PortDir    string
	CRC        uint32
	Version    string
	PkgFile    string
	Size       int64
	Mtime      int64
	BuildTime  int64
}

// Global CRC database instance
var globalCRCDB *CRCDatabase

// InitCRCDatabase initializes or loads the CRC database
func InitCRCDatabase(cfg *config.Config) (*CRCDatabase, error) {
	if globalCRCDB != nil {
		return globalCRCDB, nil
	}

	dbPath := filepath.Join(cfg.BuildBase, "dsynth.db")

	db := &CRCDatabase{
		path:    dbPath,
		entries: make(map[string]*CRCEntry),
	}

	// Try to load existing database
	if _, err := os.Stat(dbPath); err == nil {
		if err := db.Load(); err != nil {
			fmt.Printf("Warning: failed to load CRC database: %v\n", err)
			// Continue with empty database
		}
	}

	globalCRCDB = db
	return db, nil
}

// Load reads the CRC database from disk
func (db *CRCDatabase) Load() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	file, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Simple binary format:
	// For each entry:
	//   uint32: CRC
	//   uint16: portdir length
	//   []byte: portdir
	//   uint16: version length
	//   []byte: version
	//   uint16: pkgfile length
	//   []byte: pkgfile
	//   int64: size
	//   int64: mtime
	//   int64: buildtime

	for {
		var crc uint32
		if err := binary.Read(file, binary.LittleEndian, &crc); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		entry := &CRCEntry{CRC: crc}

		// Read portdir
		if s, err := readString(file); err != nil {
			return err
		} else {
			entry.PortDir = s
		}

		// Read version
		if s, err := readString(file); err != nil {
			return err
		} else {
			entry.Version = s
		}

		// Read pkgfile
		if s, err := readString(file); err != nil {
			return err
		} else {
			entry.PkgFile = s
		}

		// Read size
		if err := binary.Read(file, binary.LittleEndian, &entry.Size); err != nil {
			return err
		}

		// Read mtime
		if err := binary.Read(file, binary.LittleEndian, &entry.Mtime); err != nil {
			return err
		}

		// Read buildtime
		if err := binary.Read(file, binary.LittleEndian, &entry.BuildTime); err != nil {
			return err
		}

		db.entries[entry.PortDir] = entry
	}

	return nil
}

// Save writes the CRC database to disk
func (db *CRCDatabase) Save() error {
	db.mu.RLock()
	if !db.dirty {
		db.mu.RUnlock()
		return nil
	}
	db.mu.RUnlock()

	db.mu.Lock()
	defer db.mu.Unlock()

	// Create temp file
	tmpPath := db.path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write all entries
	for _, entry := range db.entries {
		if err := binary.Write(file, binary.LittleEndian, entry.CRC); err != nil {
			return err
		}

		if err := writeString(file, entry.PortDir); err != nil {
			return err
		}
		if err := writeString(file, entry.Version); err != nil {
			return err
		}
		if err := writeString(file, entry.PkgFile); err != nil {
			return err
		}

		if err := binary.Write(file, binary.LittleEndian, entry.Size); err != nil {
			return err
		}
		if err := binary.Write(file, binary.LittleEndian, entry.Mtime); err != nil {
			return err
		}
		if err := binary.Write(file, binary.LittleEndian, entry.BuildTime); err != nil {
			return err
		}
	}

	if err := file.Sync(); err != nil {
		return err
	}
	file.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, db.path); err != nil {
		return err
	}

	db.dirty = false
	return nil
}

// CheckNeedsBuild determines if a package needs rebuilding
func (db *CRCDatabase) CheckNeedsBuild(pkg *Package, cfg *config.Config) bool {
	db.mu.RLock()
	entry, exists := db.entries[pkg.PortDir]
	db.mu.RUnlock()

	// Always build if forced
	if cfg.Force {
		return true
	}

	// Always build if no entry exists
	if !exists {
		return true
	}

	// Check if package file exists
	pkgPath := filepath.Join(cfg.RepositoryPath, pkg.PkgFile)
	info, err := os.Stat(pkgPath)
	if os.IsNotExist(err) {
		return true
	}

	// Check if version changed
	if entry.Version != pkg.Version {
		return true
	}

	// Compute current CRC
	portPath := filepath.Join(cfg.DPortsPath, pkg.Category, pkg.Name)
	currentCRC, err := computePortCRC(portPath)
	if err != nil {
		// On error, rebuild to be safe
		return true
	}

	// Check if CRC changed
	if entry.CRC != currentCRC {
		return true
	}

	// Check if package size changed
	if entry.Size != info.Size() {
		return true
	}

	// Package is up-to-date
	return false
}

// UpdateAfterBuild updates the database after a successful build
func (db *CRCDatabase) UpdateAfterBuild(pkg *Package) {
	portPath := filepath.Join(globalCRCDB.path[:len(globalCRCDB.path)-10], "ports", pkg.Category, pkg.Name)
	crc, err := computePortCRC(portPath)
	if err != nil {
		fmt.Printf("Warning: failed to compute CRC for %s: %v\n", pkg.PortDir, err)
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.entries[pkg.PortDir] = &CRCEntry{
		PortDir:   pkg.PortDir,
		CRC:       crc,
		Version:   pkg.Version,
		PkgFile:   pkg.PkgFile,
		Size:      0, // Will be updated when package is verified
		Mtime:     time.Now().Unix(),
		BuildTime: time.Now().Unix(),
	}
	db.dirty = true
}

// Delete removes an entry from the database
func (db *CRCDatabase) Delete(portDir string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.entries[portDir]; exists {
		delete(db.entries, portDir)
		db.dirty = true
	}
}

// Stats returns statistics about the database
func (db *CRCDatabase) Stats() (total int, modified int) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	total = len(db.entries)
	if db.dirty {
		modified = 1
	}
	return
}

// computePortCRC computes a CRC32 checksum of a port directory
func computePortCRC(portPath string) (uint32, error) {
	hash := crc32.NewIEEE()

	// Walk the port directory
	err := filepath.Walk(portPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain directories and files
		base := filepath.Base(path)
		if base == ".git" || base == "work" || base == ".svn" || base == "CVS" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only hash regular files
		if !info.Mode().IsRegular() {
			return nil
		}

		// Write file path to hash (for structure)
		relPath, err := filepath.Rel(portPath, path)
		if err != nil {
			return err
		}
		hash.Write([]byte(relPath))
		hash.Write([]byte{0})

		// Write file size and mtime
		binary.Write(hash, binary.LittleEndian, info.Size())
		binary.Write(hash, binary.LittleEndian, info.ModTime().Unix())

		return nil
	})

	if err != nil {
		return 0, err
	}

	return hash.Sum32(), nil
}

// Helper functions for reading/writing strings
func readString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

func writeString(w io.Writer, s string) error {
	length := uint16(len(s))
	if err := binary.Write(w, binary.LittleEndian, length); err != nil {
		return err
	}

	_, err := w.Write([]byte(s))
	return err
}

// DeleteCRCEntry removes an entry from the CRC database
func DeleteCRCEntry(portDir string) {
	if globalCRCDB != nil {
		globalCRCDB.Delete(portDir)
	}
}

// GetCRCStats returns statistics about the CRC database
func GetCRCStats() (total int, modified int) {
	if globalCRCDB == nil {
		return 0, 0
	}
	return globalCRCDB.Stats()
}

// VerifyPackageIntegrity checks if packages match CRC database
func VerifyPackageIntegrity(cfg *config.Config) error {
	if globalCRCDB == nil {
		db, err := InitCRCDatabase(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize CRC database: %w", err)
		}
		globalCRCDB = db
	}

	fmt.Println("Verifying package integrity...")

	globalCRCDB.mu.RLock()
	entries := make([]*CRCEntry, 0, len(globalCRCDB.entries))
	for _, entry := range globalCRCDB.entries {
		entries = append(entries, entry)
	}
	globalCRCDB.mu.RUnlock()

	missing := 0
	corrupted := 0

	for i, entry := range entries {
		if i%100 == 0 {
			fmt.Printf("  Checked %d/%d packages...\r", i, len(entries))
		}

		pkgPath := filepath.Join(cfg.RepositoryPath, entry.PkgFile)
		info, err := os.Stat(pkgPath)
		if os.IsNotExist(err) {
			missing++
			fmt.Printf("\n  Missing: %s\n", entry.PortDir)
			continue
		}

		if info.Size() != entry.Size {
			corrupted++
			fmt.Printf("\n  Size mismatch: %s (expected %d, got %d)\n",
				entry.PortDir, entry.Size, info.Size())
		}
	}

	fmt.Printf("  Checked %d packages\n", len(entries))
	if missing > 0 {
		fmt.Printf("  %d packages missing\n", missing)
	}
	if corrupted > 0 {
		fmt.Printf("  %d packages corrupted\n", corrupted)
	}

	if missing == 0 && corrupted == 0 {
		fmt.Println("  All packages verified successfully")
	}

	return nil
}
