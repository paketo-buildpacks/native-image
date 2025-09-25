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

package native_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/libpak/sbom/mocks"
	"github.com/paketo-buildpacks/libpak/sherpa"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/native-image/v5/native"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx         libcnb.BuildContext
		build       native.Build
		out         bytes.Buffer
		sbomScanner mocks.SBOMScanner
	)

	it.Before(func() {
		ctx.Application.Path = t.TempDir()
		ctx.Layers.Path = t.TempDir()

		sbomScanner = mocks.SBOMScanner{}
		sbomScanner.On("ScanLaunch", ctx.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON).Return(nil)

		build.Logger = bard.NewLogger(&out)
		build.SBOMScanner = &sbomScanner

		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"dependencies": []map[string]interface{}{
				{
					"id":      "spring-graalvm-native",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
				},
			},
		}

		ctx.StackID = "test-stack-id"
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes native image layer", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

		result, err := build.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(1))
		Expect(result.Layers[0].(native.NativeImage).Arguments).To(BeEmpty())
		Expect(result.Processes).To(ContainElements(
			libcnb.Process{Type: "native-image", Command: "./test-start-class", Direct: true},
			libcnb.Process{Type: "task", Command: "./test-start-class", Direct: true},
			libcnb.Process{Type: "web", Command: "./test-start-class", Direct: true, Default: true},
		))
		sbomScanner.AssertCalled(t, "ScanLaunch", ctx.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON)
	})

	context("BP_NATIVE_IMAGE", func() {
		context("when true", func() {
			it.Before(func() {
				t.Setenv("BP_NATIVE_IMAGE", "true")
			})

			it("contributes native image layer", func() {
				Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

				result, err := build.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Layers).To(HaveLen(1))
				Expect(result.Layers[0].(native.NativeImage).Arguments).To(BeEmpty())
				Expect(result.Processes).To(ContainElements(
					libcnb.Process{Type: "native-image", Command: "./test-start-class", Direct: true},
					libcnb.Process{Type: "task", Command: "./test-start-class", Direct: true},
					libcnb.Process{Type: "web", Command: "./test-start-class", Direct: true, Default: true},
				))

				sbomScanner.AssertCalled(t, "ScanLaunch", ctx.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON)
			})
		})

		context("when false", func() {
			it.Before(func() {
				t.Setenv("BP_NATIVE_IMAGE", "false")
			})

			it("does nothing and skips build", func() {
				Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

				result, err := build.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Layers).To(HaveLen(0))
				Expect(result.Processes).To(BeEmpty())

				sbomScanner.AssertNotCalled(t, "ScanLaunch", ctx.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON)
			})
		})
	})

	context("BP_BOOT_NATIVE_IMAGE", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_BOOT_NATIVE_IMAGE", "true")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_BOOT_NATIVE_IMAGE")).To(Succeed())
		})

		it("contributes native image layer and prints a deprecation warning", func() {
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

			result, err := build.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			Expect(result.Layers[0].(native.NativeImage).Arguments).To(BeEmpty())
			Expect(result.Processes).To(ContainElements(
				libcnb.Process{Type: "native-image", Command: "./test-start-class", Direct: true},
				libcnb.Process{Type: "task", Command: "./test-start-class", Direct: true},
				libcnb.Process{Type: "web", Command: "./test-start-class", Direct: true, Default: true},
			))

			Expect(out.String()).To(ContainSubstring("$BP_BOOT_NATIVE_IMAGE has been deprecated. Please use $BP_NATIVE_IMAGE instead."))
			sbomScanner.AssertCalled(t, "ScanLaunch", ctx.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON)
		})
	})

	context("BP_NATIVE_IMAGE_BUILD_ARGUMENTS", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_NATIVE_IMAGE_BUILD_ARGUMENTS", "test-native-image-argument")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_NATIVE_IMAGE_BUILD_ARGUMENTS")).To(Succeed())
		})

		it("contributes native image build arguments", func() {
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

			result, err := build.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[0].(native.NativeImage).Arguments).To(Equal("test-native-image-argument"))
		})
	})

	context("BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS", "test-native-image-argument")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS")).To(Succeed())
		})

		it("contributes native image build arguments and prints a deprecation warning", func() {
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`
Spring-Boot-Version: 1.1.1
Spring-Boot-Classes: BOOT-INF/classes
Spring-Boot-Lib: BOOT-INF/lib
Spring-Boot-Layers-Index: layers.idx
Start-Class: test-start-class
`), 0644)).To(Succeed())

			result, err := build.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[0].(native.NativeImage).Arguments).To(Equal("test-native-image-argument"))

			Expect(out.String()).To(ContainSubstring("$BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS has been deprecated. Please use $BP_NATIVE_IMAGE_BUILD_ARGUMENTS instead."))
		})
	})

	context("BP_NATIVE_IMAGE_BUILT_ARTIFACT", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_NATIVE_IMAGE_BUILT_ARTIFACT", "target/*.jar")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_NATIVE_IMAGE_BUILT_ARTIFACT")).To(Succeed())
		})

		it("contributes native image layer to build against a JAR", func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "target"), 0755)).To(Succeed())

			fp, err := os.Open("testdata/test-fixture.jar")
			Expect(err).ToNot(HaveOccurred())
			Expect(sherpa.CopyFile(fp, filepath.Join(ctx.Application.Path, "target", "test-fixture.jar"))).To(Succeed())

			result, err := build.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[0].(native.NativeImage).JarFilePattern).To(Equal("target/*.jar"))
			Expect(result.Processes).To(ContainElements(
				libcnb.Process{Type: "native-image", Command: "./test-fixture", Direct: true},
				libcnb.Process{Type: "task", Command: "./test-fixture", Direct: true},
				libcnb.Process{Type: "web", Command: "./test-fixture", Direct: true, Default: true},
			))
		})
	})
}
