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
	"errors"
	"fmt"
	"github.com/paketo-buildpacks/libpak/sherpa"
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

	manifest, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to read manifest in %s\n%w", context.Application.Path, err)
	}

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	if _, ok := cr.Resolve(DeprecatedConfigNativeImage); ok {
		warn(b.Logger, fmt.Sprintf("$%s has been deprecated. Please use $%s instead.",
			DeprecatedConfigNativeImage,
			ConfigNativeImage,
		))
	}

	args, ok := cr.Resolve(ConfigNativeImageArgs)
	if !ok {
		if args, ok = cr.Resolve(DeprecatedConfigNativeImageArgs); ok {
			warn(b.Logger, fmt.Sprintf("$%s has been deprecated. Please use $%s instead.",
				DeprecatedConfigNativeImageArgs,
				ConfigNativeImageArgs,
			))
		}
	}

	jarFilePattern, _ := cr.Resolve("BP_NATIVE_IMAGE_BUILT_ARTIFACT")
	argsFile, _ := cr.Resolve("BP_NATIVE_IMAGE_BUILD_ARGUMENTS_FILE")

	nativeImageArgFile := filepath.Join(context.Application.Path, "META-INF", "native-image", "argfile")
	if exists, err := sherpa.Exists(nativeImageArgFile); err != nil{
		return libcnb.BuildResult{}, fmt.Errorf("unable to check for native-image arguments file at %s\n%w", nativeImageArgFile, err)
	} else if !exists{
		nativeImageArgFile = ""
	}

	compressor, ok := cr.Resolve(BinaryCompressionMethod)
	if !ok {
		compressor = CompressorNone
	} else if ok {
		if compressor != CompressorUpx && compressor != CompressorGzexe && compressor != CompressorNone {
			warn(b.Logger, fmt.Sprintf("Requested compression method [%s] is unknown, no compression will be performed", compressor))
			compressor = CompressorNone
		}
	}

	n, err := NewNativeImage(context.Application.Path, args, argsFile, nativeImageArgFile, compressor, jarFilePattern, manifest, context.StackID)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create native image layer\n%w", err)
	}
	n.Logger = b.Logger
	result.Layers = append(result.Layers, n)

	startClass, err := findStartOrMainClass(manifest, context.Application.Path, jarFilePattern)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find required manifest property\n%w", err)
	}

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
func warn(l bard.Logger, msg string) {
	l.Headerf(
		"\n%s %s\n\n",
		color.New(color.FgYellow, color.Bold).Sprintf("Warning:"),
		msg,
	)
}

func findStartOrMainClass(manifest *properties.Properties, appPath, jarFilePattern string) (string, error) {
	_, startClass, err := ExplodedJarArguments{Manifest: manifest}.Configure(nil)
	if err != nil && !errors.Is(err, NoStartOrMainClass{}) {
		return "", fmt.Errorf("unable to find startClass\n%w", err)
	}

	if startClass != "" {
		return startClass, nil
	}

	_, startClass, err = JarArguments{JarFilePattern: jarFilePattern, ApplicationPath: appPath}.Configure(nil)
	if err != nil {
		return "", fmt.Errorf("unable to find startClass from JAR\n%w", err)
	}

	if startClass != "" {
		return startClass, nil
	}

	return "", fmt.Errorf("unable to find a suitable startClass")
}
