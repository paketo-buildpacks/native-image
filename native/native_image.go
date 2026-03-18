/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package native

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/paketo-buildpacks/native-image/v5/native/slices"

	"github.com/buildpacks/libcnb"
	"github.com/magiconair/properties"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

type NativeImage struct {
	ApplicationPath string
	Arguments       string
	ArgumentsFile   string
	Executor        effect.Executor
	IncludeFiles    []string
	JarFilePattern  string
	Logger          bard.Logger
	Manifest        *properties.Properties
	StackID         string
	Compressor      string
}

func NewNativeImage(applicationPath string, arguments string, argumentsFile string, compressor string, includeFiles []string, jarFilePattern string, manifest *properties.Properties, stackID string) (NativeImage, error) {
	return NativeImage{
		ApplicationPath: applicationPath,
		Arguments:       arguments,
		ArgumentsFile:   argumentsFile,
		Executor:        effect.NewExecutor(),
		IncludeFiles:    includeFiles,
		JarFilePattern:  jarFilePattern,
		Manifest:        manifest,
		StackID:         stackID,
		Compressor:      compressor,
	}, nil
}

func (n NativeImage) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	n.Logger.Header("DEBUG: Application path contents before native-image build")
	if err := debugListDir(n.ApplicationPath, n.ApplicationPath, n.Logger, 0); err != nil {
		n.Logger.Bodyf("DEBUG: unable to list application path: %v", err)
	}

	files, err := sherpa.NewFileListing(n.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to create file listing for %s\n%w", n.ApplicationPath, err)
	}

	arguments, startClass, err := n.ProcessArguments(layer)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to process arguments\n%w", err)
	}

	if !slices.Contains(arguments, "--auto-fallback") && !slices.Contains(arguments, "--force-fallback") {
		arguments = append([]string{"--no-fallback"}, arguments...)
	}

	buf := &bytes.Buffer{}
	if err := n.Executor.Execute(effect.Execution{
		Command: "native-image",
		Args:    []string{"--version"},
		Stdout:  buf,
		Stderr:  n.Logger.BodyWriter(),
	}); err != nil {
		return libcnb.Layer{}, fmt.Errorf("error running version\n%w", err)
	}
	nativeBinaryHash := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))

	contributor := libpak.NewLayerContributor("Native Image", map[string]interface{}{
		"files":         files,
		"arguments":     arguments,
		"compression":   n.Compressor,
		"version-hash":  nativeBinaryHash,
		"include-files": n.IncludeFiles,
	}, libcnb.LayerTypes{
		Cache: true,
	})
	contributor.Logger = n.Logger

	layer, err = contributor.Contribute(layer, func() (libcnb.Layer, error) {
		n.Logger.Bodyf("Executing native-image %s", strings.Join(arguments, " "))
		if err := n.Executor.Execute(effect.Execution{
			Command: "native-image",
			Args:    arguments,
			Dir:     layer.Path,
			Stdout:  n.Logger.InfoWriter(),
			Stderr:  n.Logger.InfoWriter(),
		}); err != nil {
			return libcnb.Layer{}, fmt.Errorf("error running build\n%w", err)
		}

		if n.Compressor == CompressorUpx {
			n.Logger.Bodyf("Executing %s to compress native image", n.Compressor)
			if err := n.Executor.Execute(effect.Execution{
				Command: "upx",
				Args:    []string{"-q", "-9", filepath.Join(layer.Path, startClass)},
				Dir:     layer.Path,
				Stdout:  n.Logger.InfoWriter(),
				Stderr:  n.Logger.InfoWriter(),
			}); err != nil {
				return libcnb.Layer{}, fmt.Errorf("error compressing\n%w", err)
			}
		} else if n.Compressor == CompressorGzexe {
			n.Logger.Bodyf("Executing %s to compress native image", n.Compressor)
			if err := n.Executor.Execute(effect.Execution{
				Command: "gzexe",
				Args:    []string{filepath.Join(layer.Path, startClass)},
				Dir:     layer.Path,
				Stdout:  n.Logger.InfoWriter(),
				Stderr:  n.Logger.InfoWriter(),
			}); err != nil {
				return libcnb.Layer{}, fmt.Errorf("error compressing\n%w", err)
			}

			if err := os.Remove(filepath.Join(layer.Path, fmt.Sprintf("%s~", startClass))); err != nil {
				return libcnb.Layer{}, fmt.Errorf("error removing\n%w", err)
			}
		}

		return layer, nil
	})
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to contribute native-image layer\n%w", err)
	}

	n.Logger.Header("Removing bytecode")

	topLevelPatterns, nestedPatterns := splitPatterns(n.IncludeFiles)

	savedDir, err := saveNestedIncludes(n.ApplicationPath, nestedPatterns, n.Logger)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to save included files\n%w", err)
	}

	cs, err := os.ReadDir(n.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to list children of %s\n%w", n.ApplicationPath, err)
	}
	for _, c := range cs {
		if shouldPreserve(c.Name(), topLevelPatterns) {
			n.Logger.Bodyf("Preserving %s", c.Name())
			continue
		}
		file := filepath.Join(n.ApplicationPath, c.Name())
		if err := os.RemoveAll(file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to remove %s\n%w", file, err)
		}
	}

	if err := copyFilesFromLayer(layer.Path, startClass, n.ApplicationPath); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to copy files from layer\n%w", err)
	}

	if err := restoreNestedIncludes(savedDir, n.ApplicationPath, n.Logger); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to restore included files\n%w", err)
	}

	return layer, nil
}

func (n NativeImage) ProcessArguments(layer libcnb.Layer) ([]string, string, error) {
	var arguments []string
	var startClass string
	var err error

	arguments, _, err = BaselineArguments{StackID: n.StackID}.Configure(nil)
	if err != nil {
		return []string{}, "", fmt.Errorf("unable to set baseline arguments\n%w", err)
	}

	if n.ArgumentsFile != "" {
		arguments, _, err = UserFileArguments{ArgumentsFile: n.ArgumentsFile}.Configure(arguments)
		if err != nil {
			return []string{}, "", fmt.Errorf("unable to create user file arguments\n%w", err)
		}
	}

	arguments, _, err = UserArguments{Arguments: n.Arguments}.Configure(arguments)
	if err != nil {
		return []string{}, "", fmt.Errorf("unable to create user arguments\n%w", err)
	}

	_, err = os.Stat(filepath.Join(n.ApplicationPath, "META-INF", "MANIFEST.MF"))
	if err != nil && !os.IsNotExist(err) {
		return []string{}, "", fmt.Errorf("unable to check for manifest\n%w", err)
	} else if err != nil && os.IsNotExist(err) {
		arguments, startClass, err = JarArguments{
			ApplicationPath: n.ApplicationPath,
			JarFilePattern:  n.JarFilePattern,
		}.Configure(arguments)
		if err != nil {
			return []string{}, "", fmt.Errorf("unable to append jar arguments\n%w", err)
		}
	} else {
		arguments, startClass, err = ExplodedJarArguments{
			ApplicationPath: n.ApplicationPath,
			LayerPath:       layer.Path,
			Manifest:        n.Manifest,
		}.Configure(arguments)
		if err != nil {
			return []string{}, "", fmt.Errorf("unable to append exploded-jar directory arguments\n%w", err)
		}
	}

	return arguments, startClass, err
}

func (NativeImage) Name() string {
	return "native-image"
}

func shouldPreserve(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

// splitPatterns separates include patterns into top-level (e.g. "dynatrace")
// and nested (e.g. "target/dynatrace") based on whether they contain a path separator.
func splitPatterns(patterns []string) (topLevel []string, nested []string) {
	for _, p := range patterns {
		if strings.Contains(p, "/") {
			nested = append(nested, p)
		} else {
			topLevel = append(topLevel, p)
		}
	}
	return
}

// computeSavePath determines the relative path under the temp directory where
// a matched file should be saved. For patterns ending with * or ** (wildcard
// glob), it preserves the parent directory. For other patterns, it strips
// leading container directories.
func ComputeSavePath(pattern string, relPath string) string {
	parts := strings.Split(pattern, "/")
	lastPart := parts[len(parts)-1]

	var stripCount int
	if lastPart == "*" || lastPart == "**" {
		// Wildcard glob: preserve parent dir as part of save path
		// "dynatrace/**" → strip 0, save "dynatrace/agent"
		// "a/b/dynatrace/**" → strip 2, save "dynatrace/agent"
		stripCount = len(parts) - 2
		if stripCount < 0 {
			stripCount = 0
		}
	} else {
		// Specific pattern or literal: strip container directories
		// "target/dynatrace" → strip 1, save "dynatrace"
		// "target/dt-agent-*" → strip 1, save "dt-agent-v1"
		stripCount = len(parts) - 1
	}

	relParts := strings.Split(relPath, "/")
	if stripCount >= len(relParts) {
		stripCount = len(relParts) - 1
	}

	return filepath.Join(relParts[stripCount:]...)
}

// saveNestedIncludes moves directories matching nested patterns (e.g. "target/dynatrace")
// out of the application path into a temporary directory before bytecode removal.
// Returns the temp directory path (empty string if nothing was saved).
func saveNestedIncludes(appPath string, patterns []string, logger bard.Logger) (string, error) {
	if len(patterns) == 0 {
		return "", nil
	}

	var savedDir string
	for _, pattern := range patterns {
		fullPattern := filepath.Join(appPath, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return "", fmt.Errorf("unable to glob pattern %s\n%w", pattern, err)
		}

		for _, match := range matches {
			if savedDir == "" {
				savedDir, err = os.MkdirTemp("", "native-image-include-*")
				if err != nil {
					return "", fmt.Errorf("unable to create temp directory\n%w", err)
				}
			}

			relPath, err := filepath.Rel(appPath, match)
			if err != nil {
				return "", fmt.Errorf("unable to compute relative path for %s\n%w", match, err)
			}
			savePath := ComputeSavePath(pattern, relPath)
			dst := filepath.Join(savedDir, savePath)
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return "", fmt.Errorf("unable to create directory for %s\n%w", dst, err)
			}
			logger.Bodyf("Saving %s for inclusion", pattern)
			if err := moveAcrossDevices(match, dst); err != nil {
				return "", fmt.Errorf("unable to save %s to %s\n%w", match, dst, err)
			}
		}
	}

	return savedDir, nil
}

// restoreNestedIncludes moves previously saved directories back to the application root.
func restoreNestedIncludes(savedDir string, appPath string, logger bard.Logger) error {
	if savedDir == "" {
		return nil
	}
	defer os.RemoveAll(savedDir)

	entries, err := os.ReadDir(savedDir)
	if err != nil {
		return fmt.Errorf("unable to read saved includes from %s\n%w", savedDir, err)
	}

	for _, entry := range entries {
		src := filepath.Join(savedDir, entry.Name())
		dst := filepath.Join(appPath, entry.Name())
		logger.Bodyf("Restoring %s", entry.Name())
		if err := moveAcrossDevices(src, dst); err != nil {
			return fmt.Errorf("unable to restore %s to %s\n%w", src, dst, err)
		}
	}

	return nil
}

// copy the main file & any `*.so` files also in the layer to the application path
func copyFilesFromLayer(layerPath string, execName string, appPath string) error {
	files, err := os.ReadDir(layerPath)
	if err != nil {
		return fmt.Errorf("unable to list files on layer %s\n%w", layerPath, err)
	}

	for _, file := range files {
		if file.Type().IsRegular() && (file.Name() == execName) {
			src := filepath.Join(layerPath, file.Name())
			dst := filepath.Join(appPath, file.Name())

			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("unable to copy %s to %s\n%w", src, dst, err)
			}
		}
		if file.Type().IsRegular() && (strings.HasSuffix(file.Name(), ".so")) {
			src := filepath.Join(layerPath, file.Name())
			dst := filepath.Join(appPath, file.Name())

			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("unable to copy %s to %s\n%w", src, dst, err)
			}
		}
	}

	return nil
}

func debugListDir(root string, dir string, logger bard.Logger, depth int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		rel, _ := filepath.Rel(root, filepath.Join(dir, e.Name()))
		prefix := strings.Repeat("  ", depth)
		if e.IsDir() {
			logger.Bodyf("%s%s/", prefix, rel)
			if depth < 3 {
				if err := debugListDir(root, filepath.Join(dir, e.Name()), logger, depth+1); err != nil {
					return err
				}
			}
		} else {
			logger.Bodyf("%s%s", prefix, rel)
		}
	}
	return nil
}

// moveAcrossDevices attempts os.Rename first; if it fails with EXDEV
// (cross-device link), it falls back to a recursive copy then delete.
func moveAcrossDevices(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := copyDirRecursive(src, dst); err != nil {
			return err
		}
	} else {
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	return os.RemoveAll(src)
}

func copyDirRecursive(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyFileWithMode(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFileWithMode(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", src, err)
	}
	defer in.Close()

	if err := sherpa.CopyFile(in, dst); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", src, err)
	}
	defer in.Close()

	return sherpa.CopyFile(in, dst)
}
