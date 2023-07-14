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
	"fmt"
	"github.com/paketo-buildpacks/native-image/v5/native/slices"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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
	JarFilePattern  string
	Logger          bard.Logger
	Manifest        *properties.Properties
	StackID         string
	Compressor      string
}

func NewNativeImage(applicationPath string, arguments string, argumentsFile string, compressor string, jarFilePattern string, manifest *properties.Properties, stackID string) (NativeImage, error) {
	return NativeImage{
		ApplicationPath: applicationPath,
		Arguments:       arguments,
		ArgumentsFile:   argumentsFile,
		Executor:        effect.NewExecutor(),
		JarFilePattern:  jarFilePattern,
		Manifest:        manifest,
		StackID:         stackID,
		Compressor:      compressor,
	}, nil
}

func (n NativeImage) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
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

	moduleVar := "USE_NATIVE_IMAGE_JAVA_PLATFORM_MODULE_SYSTEM"
	if _, set := os.LookupEnv(moduleVar); !set {
		if err := os.Setenv(moduleVar, "false"); err != nil {
			n.Logger.Bodyf("unable to set %s for GraalVM 22.2, if your build fails, you may need to set this manually at build time", moduleVar)
		}
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
		"files":        files,
		"arguments":    arguments,
		"compression":  n.Compressor,
		"version-hash": nativeBinaryHash,
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
	cs, err := ioutil.ReadDir(n.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to list children of %s\n%w", n.ApplicationPath, err)
	}
	for _, c := range cs {
		file := filepath.Join(n.ApplicationPath, c.Name())
		if err := os.RemoveAll(file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to remove %s\n%w", file, err)
		}
	}

	/**
	 * native-image with -g since 22.3 splits debug info in separate file
	 */
	compiled, err := ioutil.ReadDir(layer.Path)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to list children of %s\n%w", n.ApplicationPath, err)
	}
	for _, file := range compiled {
		src := filepath.Join(layer.Path, file.Name())
		in, err := os.Open(src)
		fileInfo, err := in.Stat()
		if fileInfo.IsDir() {
			/*TODO: for now skip directories, but perhaps it's better to zip its content */
			continue
		}
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to open %s\n%w", filepath.Join(layer.Path, startClass), err)
		}
		defer in.Close()

		dst := filepath.Join(n.ApplicationPath, file.Name())
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to open %s\n%w", dst, err)
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to copy %s -> %s \n%w", in.Name(), out.Name(), err)
		}

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
