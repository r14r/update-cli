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

var semverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type Version struct{ Major, Minor, Patch int }

func Parse(s string) (Version, error) {
	m := semverPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, fmt.Errorf("ungültige semantische Version %q; erwartet wird 1.2.3", s)
	}
	v := Version{}
	vals := []*int{&v.Major, &v.Minor, &v.Patch}
	for i := range vals {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, err
		}
		*vals[i] = n
	}
	return v, nil
}
func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }
func (v Version) Compare(o Version) int {
	a := []int{v.Major, v.Minor, v.Patch}
	b := []int{o.Major, o.Minor, o.Patch}
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
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

func ParseArchiveName(project, filename string) (Version, error) {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(project) + `-v(\d+\.\d+\.\d+)\.zip$`)
	m := re.FindStringSubmatch(filepath.Base(filename))
	if m == nil {
		return Version{}, fmt.Errorf("ungültiger Archivname %q; erwartet wird %s-v1.2.3.zip", filepath.Base(filename), project)
	}
	return Parse(m[1])
}
func ListArchives(dir, project string) ([]ArchiveInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Download-Ordner kann nicht gelesen werden: %w", err)
	}
	out := []ArchiveInfo{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		v, err := ParseArchiveName(project, e.Name())
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		p, _ := filepath.Abs(filepath.Join(dir, e.Name()))
		out = append(out, ArchiveInfo{Path: p, Name: e.Name(), Version: v, VersionS: v.String(), Size: info.Size(), Modified: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool {
		c := out[i].Version.Compare(out[j].Version)
		if c == 0 {
			return out[i].Modified.After(out[j].Modified)
		}
		return c > 0
	})
	return out, nil
}
func SelectNewest(dir, project string) (string, Version, error) {
	a, err := ListArchives(dir, project)
	if err != nil {
		return "", Version{}, err
	}
	if len(a) == 0 {
		return "", Version{}, fmt.Errorf("keine passende ZIP-Datei in %s gefunden; erwartet wird %s-v<MAJOR>.<MINOR>.<PATCH>.zip", dir, project)
	}
	return a[0].Path, a[0].Version, nil
}
