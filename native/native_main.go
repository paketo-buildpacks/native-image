package native

import (
	"fmt"
	"github.com/magiconair/properties"
	"os"
	"path/filepath"
	"strings"
)

type nativeMain interface {
	Name() (string, error)
	Arguments() []string
	ClassPath() string
}

type startClassMain struct {
	ApplicationPath string
	Manifest  *properties.Properties
	startClass string
}

type jarFileMain struct {
	jarFileName		string
	jarFile			string
	jarFilePath		string
}

func newStartClassMain(path string, manifest  *properties.Properties) *startClassMain {
	return &startClassMain{
		ApplicationPath: path,
		Manifest: manifest,
	}
}

func (m *startClassMain) Name() (string, error) {
	var err error
	if m.startClass == "" {
		m.startClass, err = findStartOrMainClass(m.Manifest)
		if err != nil {
			return "", fmt.Errorf("unable to find required manifest property\n%w", err)
		}
	}
	return m.startClass, nil
}

func (m *startClassMain) Arguments() []string {
	return []string {m.startClass}
}

func (m *startClassMain) ClassPath() string {
	cp := os.Getenv("CLASSPATH")
	if cp == "" {
		// CLASSPATH should have been done by upstream buildpacks, but just in case
		cp = m.ApplicationPath
		if v, ok := m.Manifest.Get("Class-Path"); ok {
			cp = strings.Join([]string{cp, v}, string(filepath.ListSeparator))
		}
	}
	return cp
}

func newJarFileMain (path string, file string) (*jarFileMain, error) {
	jarFile :=  filepath.Base(file)
	jarFilePath := filepath.Join(path, filepath.Dir(file))
	if ".jar" != filepath.Ext(jarFile) {
		return &jarFileMain{}, fmt.Errorf("file %s has not a jar extension\n", jarFile)
	}
	jarFileName := strings.TrimSuffix(jarFile, ".jar")
	return &jarFileMain{
		jarFileName: jarFileName,
		jarFile:     jarFile,
		jarFilePath: jarFilePath,
	}, nil
}

func (m *jarFileMain) Name() (string, error ) {
	return m.jarFileName, nil
}

func (m *jarFileMain) Arguments() []string {
	return []string {"-jar", filepath.Join(m.jarFilePath, m.jarFile)}
}

func (m *jarFileMain) ClassPath() string {
	return fmt.Sprintf("%s:/%s", m.jarFilePath, "lib")
}

func findStartOrMainClass(manifest *properties.Properties) (string, error) {
	startClass, ok := manifest.Get("Start-Class")
	if !ok {
		startClass, ok = manifest.Get("Main-Class")
		if !ok {
			return "", fmt.Errorf("unable to read Start-Class or Main-Class from MANIFEST.MF")
		}
	}
	return startClass, nil
}
