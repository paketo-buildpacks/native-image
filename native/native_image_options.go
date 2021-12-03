package native

import (
	"fmt"
	"github.com/magiconair/properties"
	"os"
	"path/filepath"
	"strings"
)

type NativeImageOption func(*options) error

type options struct {
	jarFileName	string
}

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

func WithJar(fileName string) NativeImageOption {
	return func(i *options) error {
		i.jarFileName = fileName
		return nil
	}
}

func newJarFileMain (path string, fileName string) *jarFileMain {
	jarFile :=   filepath.Join(path, fmt.Sprintf("%s.jar", fileName))
	return &jarFileMain{
		jarFileName: fileName,
		jarFile:     jarFile,
	}
}

func (m *jarFileMain) Name() (string, error ) {
	return m.jarFileName, nil
}

func (m *jarFileMain) Arguments() []string {
	return []string {"-jar", m.jarFile}
}

func (m *jarFileMain) ClassPath() string {
	// TODO I still have doubts if this could defined by user or determined programmatically
	return "workspace:/lib"
}
