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
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/magiconair/properties"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/native-image/v5/native"
	"github.com/sclevine/spec"
)

func testArguments(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx   libcnb.BuildContext
		props *properties.Properties
	)

	it.Before(func() {
		ctx.Application.Path = t.TempDir()
		ctx.Layers.Path = t.TempDir()
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	context("baseline arguments", func() {
		it("sets default arguments", func() {
			args, startClass, err := native.BaselineArguments{}.Configure(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(0))
		})

		it("ignores input arguments", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.BaselineArguments{}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(0))
		})

		it("sets defaults for tiny stack", func() {
			args, startClass, err := native.BaselineArguments{StackID: libpak.TinyStackID}.Configure(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(1))
			Expect(args).To(Equal([]string{"-H:+StaticExecutableWithDynamicLibC"}))
		})
	})

	context("user arguments", func() {
		it("has none", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.UserArguments{}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(3))
			Expect(args).To(Equal([]string{"one", "two", "three"}))
		})

		it("has some and appends to end", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.UserArguments{
				Arguments: "more stuff",
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(5))
			Expect(args).To(Equal([]string{"one", "two", "three", "more", "stuff"}))
		})

		it("works with quotes", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.UserArguments{
				Arguments: `"more stuff"`,
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(4))
			Expect(args).To(Equal([]string{"one", "two", "three", "more stuff"}))
		})

		it("allows a user argument to override an input argument", func() {
			inputArgs := []string{"one=input", "two", "three"}
			args, startClass, err := native.UserArguments{
				Arguments: `one=output`,
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(3))
			Expect(args).To(Equal([]string{"two", "three", "one=output"}))
		})
	})

	context("user arguments from file", func() {
		it.Before(func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "target"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "more-stuff.txt"), []byte("more stuff"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "more-stuff-quotes.txt"), []byte(`before -jar "more stuff.jar" after -other="my path"`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "more-stuff-class.txt"), []byte(`stuff -jar stuff.jar after`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "override.txt"), []byte(`one=output`), 0644)).To(Succeed())
		})

		it("has none", func() {
			inputArgs := []string{"one", "two", "three"}
			_, _, err := native.UserFileArguments{}.Configure(inputArgs)
			Expect(err).To(MatchError(os.ErrNotExist))
		})

		it("has some and appends to end", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.UserFileArguments{
				ArgumentsFile: filepath.Join(ctx.Application.Path, "target/more-stuff.txt"),
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(4))
			Expect(args).To(Equal([]string{"one", "two", "three", fmt.Sprintf("@%s", filepath.Join(ctx.Application.Path, "target/more-stuff.txt"))}))
		})

		it("works with quotes in the file", func() {
			inputArgs := []string{"one", "two", "three"}
			args, startClass, err := native.UserFileArguments{
				ArgumentsFile: filepath.Join(ctx.Application.Path, "target/more-stuff-quotes.txt"),
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal(""))
			Expect(args).To(HaveLen(4))
			Expect(args).To(Equal([]string{"one", "two", "three", fmt.Sprintf("@%s", filepath.Join(ctx.Application.Path, "target/more-stuff-quotes.txt"))}))
			bits, err := os.ReadFile(filepath.Join(ctx.Application.Path, "target/more-stuff-quotes.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(bits)).To(Equal("before after -other=\"my path\""))
		})

		it("removes the class name argument if found", func() {
			args, _, err := native.UserFileArguments{
				ArgumentsFile: filepath.Join(ctx.Application.Path, "target/more-stuff-class.txt"),
			}.Configure(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(HaveLen(1))
			Expect(args).To(Equal([]string{
				fmt.Sprintf("@%s", filepath.Join(ctx.Application.Path, "target", "more-stuff-class.txt")),
			}))
			bits, err := os.ReadFile(filepath.Join(ctx.Application.Path, "target/more-stuff-class.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(bits)).To(Equal("after"))
		})
	})

	context("exploded jar arguments", func() {
		var layer libcnb.Layer

		it.Before(func() {
			var err error

			layer, err = ctx.Layers.Layer("test-layer")
			Expect(err).NotTo(HaveOccurred())

			props = properties.NewProperties()
			_, _, err = props.Set("Start-Class", "test-start-class")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = props.Set("Class-Path", "manifest-class-path")
			Expect(err).NotTo(HaveOccurred())
		})

		it("adds arguments, no CLASSPATH set", func() {
			inputArgs := []string{"stuff"}
			args, startClass, err := native.ExplodedJarArguments{
				ApplicationPath: ctx.Application.Path,
				LayerPath:       layer.Path,
				Manifest:        props,
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal("test-start-class"))
			Expect(args).To(HaveLen(5))
			Expect(args).To(Equal([]string{
				"stuff",
				fmt.Sprintf("-H:Name=%s/test-start-class", layer.Path),
				"-cp",
				fmt.Sprintf("%s:%s", ctx.Application.Path, "manifest-class-path"),
				"test-start-class"}))
		})

		it("fails to find start or main class", func() {
			inputArgs := []string{"stuff"}
			_, _, err := native.ExplodedJarArguments{
				ApplicationPath: ctx.Application.Path,
				LayerPath:       layer.Path,
				Manifest:        properties.NewProperties(),
			}.Configure(inputArgs)
			Expect(err).To(MatchError("unable to read Start-Class or Main-Class from MANIFEST.MF"))
		})

		context("CLASSPATH is set", func() {
			it.Before(func() {
				Expect(os.Setenv("CLASSPATH", "some-classpath")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("CLASSPATH")).To(Succeed())
			})

			it("adds arguments", func() {
				inputArgs := []string{"stuff"}
				args, startClass, err := native.ExplodedJarArguments{
					ApplicationPath: ctx.Application.Path,
					LayerPath:       layer.Path,
					Manifest:        props,
				}.Configure(inputArgs)
				Expect(err).ToNot(HaveOccurred())
				Expect(startClass).To(Equal("test-start-class"))
				Expect(args).To(HaveLen(5))
				Expect(args).To(Equal([]string{
					"stuff",
					fmt.Sprintf("-H:Name=%s/test-start-class", layer.Path),
					"-cp",
					"some-classpath",
					"test-start-class"}))
			})
		})
	})

	context("jar file", func() {
		it.Before(func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "target"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "found.jar"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "a.two"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "target", "b.two"), []byte{}, 0644)).To(Succeed())
		})

		it("adds arguments", func() {
			inputArgs := []string{"stuff"}
			args, startClass, err := native.JarArguments{
				ApplicationPath: ctx.Application.Path,
				JarFilePattern:  "target/*.jar",
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal("found"))
			Expect(args).To(HaveLen(3))
			Expect(args).To(Equal([]string{
				"stuff",
				"-jar",
				filepath.Join(ctx.Application.Path, "target", "found.jar"),
			}))
		})

		it("overrides -jar arguments", func() {
			inputArgs := []string{"stuff", "-jar", "no-where"}
			args, startClass, err := native.JarArguments{
				ApplicationPath: ctx.Application.Path,
				JarFilePattern:  "target/*.jar",
			}.Configure(inputArgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(startClass).To(Equal("found"))
			Expect(args).To(HaveLen(3))
			Expect(args).To(Equal([]string{
				"stuff",
				"-jar",
				filepath.Join(ctx.Application.Path, "target", "found.jar"),
			}))
		})

		it("pattern doesn't match", func() {
			inputArgs := []string{"stuff"}
			_, _, err := native.JarArguments{
				ApplicationPath: ctx.Application.Path,
				JarFilePattern:  "target/*.junk",
			}.Configure(inputArgs)
			Expect(err).To(MatchError("unable to find single JAR in target/*.junk, candidates: []"))
		})

		it("pattern matches multiple", func() {
			inputArgs := []string{"stuff"}
			_, _, err := native.JarArguments{
				ApplicationPath: ctx.Application.Path,
				JarFilePattern:  "target/*.two",
			}.Configure(inputArgs)
			Expect(err).To(MatchError(MatchRegexp(`unable to find single JAR in target/\*\.two, candidates: \[.*/target/a\.two .*/target/b\.two\]`)))
		})
	})
}
