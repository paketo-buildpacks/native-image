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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/magiconair/properties"
	"github.com/mattn/go-shellwords"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

type NativeImage struct {
	ApplicationPath string
	Arguments       []string
	Executor        effect.Executor
	Logger          bard.Logger
	nativeMain 		nativeMain
	StackID         string
	Compressor      string
}

func NewNativeImage(applicationPath string, arguments string, compressor string, manifest *properties.Properties, stackID string, jarFile string) (NativeImage, error) {
	args, err := shellwords.Parse(arguments)
	if err != nil {
		return NativeImage{}, fmt.Errorf("unable to parse arguments from %s\n%w", arguments, err)
	}

	var nativeMain nativeMain
	nativeMain = newStartClassMain(applicationPath, manifest)
	if jarFile != "" {
		nativeMain, err = newJarFileMain(applicationPath, jarFile)
		if err != nil {
			return NativeImage{}, fmt.Errorf("unable to parse the native jar file\n%w", err)
		}
	}

	return NativeImage{
		ApplicationPath: applicationPath,
		Arguments:       args,
		Executor:        effect.NewExecutor(),
		nativeMain:      nativeMain,
		StackID:         stackID,
		Compressor:      compressor,
	}, nil
}

func (n NativeImage) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	name, err := n.nativeMain.Name()
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to determine main class\n%w", err)
	}

	arguments := n.Arguments

	if n.StackID == libpak.TinyStackID {
		arguments = append(arguments, "-H:+StaticExecutableWithDynamicLibC")
	}

	cp := n.nativeMain.ClassPath()

	arguments = append(arguments,
		fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, name)),
		"-cp", cp,
	)
	arguments = append(arguments, n.nativeMain.Arguments()...)

	files, err := sherpa.NewFileListing(n.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to create file listing for %s\n%w", n.ApplicationPath, err)
	}

	contributor := libpak.NewLayerContributor("Native Image", map[string]interface{}{
		"files":       files,
		"arguments":   arguments,
		"compression": n.Compressor,
	}, libcnb.LayerTypes{
		Cache: true,
	})
	contributor.Logger = n.Logger

	layer, err = contributor.Contribute(layer, func() (libcnb.Layer, error) {
		if err := n.Executor.Execute(effect.Execution{
			Command: "native-image",
			Args:    []string{"--version"},
			Dir:     layer.Path,
			Stdout:  n.Logger.BodyWriter(),
			Stderr:  n.Logger.BodyWriter(),
		}); err != nil {
			return libcnb.Layer{}, fmt.Errorf("error running version\n%w", err)
		}

		n.Logger.Bodyf("Executing native-image %s", strings.Join(arguments, " "))
		fmt.Printf("----- ARGUMENTS %s\n", strings.Join(arguments, " "))
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
				Args:    []string{"-q", "-9", filepath.Join(layer.Path, name)},
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
				Args:    []string{filepath.Join(layer.Path, name)},
				Dir:     layer.Path,
				Stdout:  n.Logger.InfoWriter(),
				Stderr:  n.Logger.InfoWriter(),
			}); err != nil {
				return libcnb.Layer{}, fmt.Errorf("error compressing\n%w", err)
			}

			if err := os.Remove(filepath.Join(layer.Path, fmt.Sprintf("%s~", name))); err != nil {
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

	src := filepath.Join(layer.Path, name)
	in, err := os.Open(src)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to open %s\n%w", filepath.Join(layer.Path, name), err)
	}
	defer in.Close()

	dst := filepath.Join(n.ApplicationPath, name)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to open %s\n%w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to copy\n%w", err)
	}

	return layer, nil
}

func (NativeImage) Name() string {
	return "native-image"
}
