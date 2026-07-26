package version

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var semanticVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) Compare(other Version) int {
	left := [...]int{v.Major, v.Minor, v.Patch}
	right := [...]int{other.Major, other.Minor, other.Patch}
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

type ArchiveInfo struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Version  Version   `json:"-"`
	VersionS string    `json:"version"`
	Size     int64     `json:"sizeBytes"`
	Modified time.Time `json:"modifiedAt"`
}

func Parse(value string) (Version, error) {
	value = strings.TrimSpace(value)
	match := semanticVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return Version{}, fmt.Errorf("ungültige semantische Version %q; erwartet wird 1.2.3", value)
	}

	parts := [3]int{}
	for index := range parts {
		parsed, err := strconv.Atoi(match[index+1])
		if err != nil {
			return Version{}, fmt.Errorf("ungültige Versionsnummer: %w", err)
		}
		parts[index] = parsed
	}
	return Version{Major: parts[0], Minor: parts[1], Patch: parts[2]}, nil
}

func ParseArchiveName(projectName, filename string) (Version, error) {
	pattern := regexp.MustCompile(
		`^` + regexp.QuoteMeta(projectName) + `-v(\d+\.\d+\.\d+)\.zip$`,
	)
	match := pattern.FindStringSubmatch(filepath.Base(filename))
	if match == nil {
		return Version{}, fmt.Errorf(
			"ungültiger Archivname %q; erwartet wird %s-v1.2.3.zip",
			filepath.Base(filename),
			projectName,
		)
	}
	return Parse(match[1])
}

func ListArchives(downloadDir, projectName string) ([]ArchiveInfo, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("Download-Ordner kann nicht gelesen werden: %w", err)
	}

	archives := make([]ArchiveInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		candidateVersion, parseErr := ParseArchiveName(projectName, entry.Name())
		if parseErr != nil {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, fmt.Errorf("Archivinformationen können nicht gelesen werden (%s): %w", entry.Name(), infoErr)
		}
		path, absErr := filepath.Abs(filepath.Join(downloadDir, entry.Name()))
		if absErr != nil {
			return nil, absErr
		}
		archives = append(archives, ArchiveInfo{
			Path:     path,
			Name:     entry.Name(),
			Version:  candidateVersion,
			VersionS: candidateVersion.String(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		comparison := archives[i].Version.Compare(archives[j].Version)
		if comparison == 0 {
			return archives[i].Modified.After(archives[j].Modified)
		}
		return comparison > 0
	})
	return archives, nil
}

func SelectNewest(downloadDir, projectName string) (string, Version, error) {
	archives, err := ListArchives(downloadDir, projectName)
	if err != nil {
		return "", Version{}, err
	}
	if len(archives) == 0 {
		return "", Version{}, fmt.Errorf(
			"keine passende ZIP-Datei in %s gefunden; erwartet wird %s-v<MAJOR>.<MINOR>.<PATCH>.zip",
			downloadDir,
			projectName,
		)
	}
	return archives[0].Path, archives[0].Version, nil
}
