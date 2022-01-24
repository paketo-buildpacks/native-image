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
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sbom"
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
	Logger      bard.Logger
	SBOMScanner sbom.SBOMScanner
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	jarFile, buildFromJar := cr.Resolve("BP_NATIVE_IMAGE_BUILD_JAR");
	var manifest *properties.Properties
	if !buildFromJar {
		manifest, err = libjvm.NewManifest(context.Application.Path)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to read manifest in %s\n%w", context.Application.Path, err)
		}
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

	n, err := NewNativeImage(context.Application.Path, args, compressor, manifest, context.StackID, jarFile)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create native image layer\n%w", err)
	}

	startClass, err := n.nativeMain.Name()
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to determine the main or start class\n%w", err)
	}

	n.Logger = b.Logger
	result.Layers = append(result.Layers, n)

	command := filepath.Join(context.Application.Path, startClass)
	result.Processes = append(result.Processes,
		libcnb.Process{Type: "native-image", Command: command, Direct: true},
		libcnb.Process{Type: "task", Command: command, Direct: true},
		libcnb.Process{Type: "web", Command: command, Direct: true, Default: true},
	)

	if b.SBOMScanner == nil {
		b.SBOMScanner = sbom.NewSyftCLISBOMScanner(context.Layers, effect.NewExecutor(), b.Logger)
	}
	if err := b.SBOMScanner.ScanLaunch(context.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create Build SBoM \n%w", err)
	}

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
