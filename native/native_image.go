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
	Manifest        *properties.Properties
	StackID         string
}

func NewNativeImage(applicationPath string, arguments string, manifest *properties.Properties, stackID string) (NativeImage, error) {
	var err error

	args, err := shellwords.Parse(arguments)
	if err != nil {
		return NativeImage{}, fmt.Errorf("unable to parse arguments from %s\n%w", arguments, err)
	}

	return NativeImage{
		ApplicationPath: applicationPath,
		Arguments:       args,
		Executor:        effect.NewExecutor(),
		Manifest:        manifest,
		StackID:         stackID,
	}, nil
}

func (n NativeImage) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	startClass, ok := n.Manifest.Get("Start-Class")
	if !ok {
		return libcnb.Layer{}, fmt.Errorf("manifest does not contain Start-Class")
	}

	arguments := n.Arguments

	if n.StackID == libpak.TinyStackID {
		arguments = append(arguments, "-H:+StaticExecutableWithDynamicLibC")
	}

	cp := os.Getenv("CLASSPATH")
	if cp == "" {
		// CLASSPATH should have been done by upstream buildpacks, but just in case
		cp = n.ApplicationPath
		if v, ok := n.Manifest.Get("Class-Path"); ok {
			cp = strings.Join([]string{cp, v}, string(filepath.ListSeparator))
		}
	}

	arguments = append(arguments,
		fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, startClass)),
		"-cp", cp,
		startClass,
	)

	files, err := sherpa.NewFileListing(n.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to create file listing for %s\n%w", n.ApplicationPath, err)
	}

	contributor := libpak.NewLayerContributor("Native Image", map[string]interface{}{
		"files":     files,
		"arguments": arguments,
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
		if err := n.Executor.Execute(effect.Execution{
			Command: "native-image",
			Args:    arguments,
			Dir:     layer.Path,
			Stdout:  n.Logger.InfoWriter(),
			Stderr:  n.Logger.InfoWriter(),
		}); err != nil {
			return libcnb.Layer{}, fmt.Errorf("error running build\n%w", err)
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

	src := filepath.Join(layer.Path, startClass)
	in, err := os.Open(src)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to open %s\n%w", filepath.Join(layer.Path, startClass), err)
	}
	defer in.Close()

	dst := filepath.Join(n.ApplicationPath, startClass)
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
