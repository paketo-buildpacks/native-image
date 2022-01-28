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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/magiconair/properties"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	"github.com/paketo-buildpacks/native-image/native"
)

func testNativeImage(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx         libcnb.BuildContext
		executor    *mocks.Executor
		props       *properties.Properties
		nativeImage native.NativeImage
		layer       libcnb.Layer
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "native-image-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Layers.Path, err = ioutil.TempDir("", "native-image-layers")
		Expect(err).NotTo(HaveOccurred())

		executor = &mocks.Executor{}

		props = properties.NewProperties()

		_, _, err = props.Set("Start-Class", "test-start-class")
		Expect(err).NotTo(HaveOccurred())
		_, _, err = props.Set("Class-Path", "manifest-class-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "fixture-marker"), []byte{}, 0644)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "BOOT-INF"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte{}, 0644)).To(Succeed())

		nativeImage, err = native.NewNativeImage(ctx.Application.Path, "test-argument-1 test-argument-2", "none", "", props, ctx.StackID)
		nativeImage.Logger = bard.NewLogger(io.Discard)
		Expect(err).NotTo(HaveOccurred())
		nativeImage.Executor = executor

		executor.On("Execute", mock.MatchedBy(func(e effect.Execution) bool {
			return e.Command == "native-image" && len(e.Args) == 1 && e.Args[0] == "--version"
		})).Run(func(args mock.Arguments) {
			exec := args.Get(0).(effect.Execution)
			_, err := exec.Stdout.Write([]byte("1.2.3"))
			Expect(err).To(Succeed())
		}).Return(nil)

		executor.On("Execute", mock.MatchedBy(func(e effect.Execution) bool {
			return e.Command == "native-image" &&
				(e.Args[0] == "test-argument-1" || (e.Args[0] == "-H:+StaticExecutableWithDynamicLibC" && e.Args[1] == "test-argument-1"))
		})).Run(func(args mock.Arguments) {
			exec := args.Get(0).(effect.Execution)
			lastArg := exec.Args[len(exec.Args)-1]
			Expect(ioutil.WriteFile(filepath.Join(layer.Path, lastArg), []byte{}, 0644)).To(Succeed())
		}).Return(nil)

		layer, err = ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	context("CLASSPATH is set", func() {
		it.Before(func() {
			Expect(os.Setenv("CLASSPATH", "some-classpath")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("CLASSPATH")).To(Succeed())
		})

		it("contributes native image", func() {
			_, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Args).To(Equal([]string{
				"test-argument-1",
				"test-argument-2",
				fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
				"-cp", "some-classpath",
				"test-start-class",
			}))
		})
	})

	context("CLASSPATH is not set", func() {
		it("contributes native image with Class-Path from manifest", func() {
			_, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Args).To(Equal([]string{
				"test-argument-1",
				"test-argument-2",
				fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
				"-cp",
				strings.Join([]string{
					ctx.Application.Path,
					"manifest-class-path",
				}, ":"),
				"test-start-class",
			}))
		})
	})

	context("Not a Spring Boot app", func() {
		it.Before(func() {
			// there won't be a Start-Class
			props.Delete("Start-Class")

			// we do expect a Main-Class
			_, _, err := props.Set("Main-Class", "test-main-class")
			Expect(err).NotTo(HaveOccurred())
		})

		it("contributes native image using Main-Class", func() {
			_, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Args).To(Equal([]string{
				"test-argument-1",
				"test-argument-2",
				fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-main-class")),
				"-cp",
				strings.Join([]string{
					ctx.Application.Path,
					"manifest-class-path",
				}, ":"),
				"test-main-class",
			}))
		})
	})

	context("upx compression is used", func() {
		it("contributes native image and runs compression", func() {
			nativeImage.Compressor = "upx"

			executor.On("Execute", mock.MatchedBy(func(e effect.Execution) bool {
				return e.Command == "upx"
			})).Run(func(args mock.Arguments) {
				Expect(ioutil.WriteFile(filepath.Join(layer.Path, "test-start-class"), []byte("upx-compressed"), 0644)).To(Succeed())
			}).Return(nil)

			_, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("native-image"))

			execution = executor.Calls[2].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("upx"))

			bin := filepath.Join(layer.Path, "test-start-class")
			Expect(bin).To(BeARegularFile())

			data, err := ioutil.ReadFile(bin)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(ContainSubstring("upx-compressed"))
		})
	})

	context("gzexe compression is used", func() {
		it("contributes native image and runs compression", func() {
			nativeImage.Compressor = "gzexe"

			executor.On("Execute", mock.MatchedBy(func(e effect.Execution) bool {
				return e.Command == "gzexe"
			})).Run(func(args mock.Arguments) {
				Expect(ioutil.WriteFile(filepath.Join(layer.Path, "test-start-class"), []byte("gzexe-compressed"), 0644)).To(Succeed())
				Expect(ioutil.WriteFile(filepath.Join(layer.Path, "test-start-class~"), []byte("original"), 0644)).To(Succeed())
			}).Return(nil)

			_, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("native-image"))

			execution = executor.Calls[2].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("gzexe"))

			bin := filepath.Join(layer.Path, "test-start-class")
			Expect(bin).To(BeARegularFile())

			data, err := ioutil.ReadFile(bin)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(ContainSubstring("gzexe-compressed"))
			Expect(filepath.Join(layer.Path, "test-start-class~")).ToNot(BeAnExistingFile())
		})
	})

	context("tiny stack", func() {
		it.Before(func() {
			nativeImage.StackID = libpak.TinyStackID
		})

		it("contributes a static native image executable with dynamic libc", func() {
			Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"
- "spring-graalvm-native-0.8.6-xxxxxx.jar"
`), 0644)).To(Succeed())
			var err error
			layer, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(layer.Cache).To(BeTrue())
			Expect(filepath.Join(layer.Path, "test-start-class")).To(BeARegularFile())
			Expect(filepath.Join(ctx.Application.Path, "test-start-class")).To(BeARegularFile())
			Expect(filepath.Join(ctx.Application.Path, "fixture-marker")).NotTo(BeAnExistingFile())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("native-image"))
			Expect(execution.Args).To(Equal([]string{
				"-H:+StaticExecutableWithDynamicLibC",
				"test-argument-1",
				"test-argument-2",
				fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
				"-cp",
				strings.Join([]string{
					ctx.Application.Path,
					"manifest-class-path",
				}, ":"),
				"test-start-class",
			}))
			Expect(execution.Dir).To(Equal(layer.Path))
		})
	})
}
