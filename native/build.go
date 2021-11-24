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
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/magiconair/properties"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

const (
	ConfigNativeImageArgs           = "BP_NATIVE_IMAGE_BUILD_ARGUMENTS"
	DeprecatedConfigNativeImageArgs = "BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS"
	CompressorUpx                   = "upx"
	CompressorGzexe                 = "gzexe"
	CompressorNone                  = "none"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	manifest, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to read manifest in %s\n%w", context.Application.Path, err)
	}

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}
	if _, ok := cr.Resolve(DeprecatedConfigNativeImage); ok {
		b.warn(fmt.Sprintf("$%s has been deprecated. Please use $%s instead.",
			DeprecatedConfigNativeImage,
			ConfigNativeImage,
		))
	}

	args, ok := cr.Resolve(ConfigNativeImageArgs)
	if !ok {
		if args, ok = cr.Resolve(DeprecatedConfigNativeImageArgs); ok {
			b.warn(fmt.Sprintf("$%s has been deprecated. Please use $%s instead.",
				DeprecatedConfigNativeImageArgs,
				ConfigNativeImageArgs,
			))
		}
	}

	compressor, ok := cr.Resolve(BinaryCompressionMethod)
	if !ok {
		compressor = CompressorNone
	} else if ok {
		if compressor != CompressorUpx && compressor != CompressorGzexe && compressor != CompressorNone {
			b.warn(fmt.Sprintf("Requested compression method [%s] is unknown, no compression will be performed", compressor))
			compressor = CompressorNone
		}
	}

	n, err := NewNativeImage(context.Application.Path, args, compressor, manifest, context.StackID)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create native image layer\n%w", err)
	}
	n.Logger = b.Logger
	result.Layers = append(result.Layers, n)

	startClass, err := findStartOrMainClass(manifest)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find required manifest property\n%w", err)
	}

	command := filepath.Join(context.Application.Path, startClass)
	result.Processes = append(result.Processes,
		libcnb.Process{Type: "native-image", Command: command, Direct: true},
		libcnb.Process{Type: "task", Command: command, Direct: true},
		libcnb.Process{Type: "web", Command: command, Direct: true, Default: true},
	)

	return result, nil
}

// todo: move warn method to the logger
func (b Build) warn(msg string) {
	b.Logger.Headerf(
		"\n%s %s\n\n",
		color.New(color.FgYellow, color.Bold).Sprintf("Warning:"),
		msg,
	)
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
