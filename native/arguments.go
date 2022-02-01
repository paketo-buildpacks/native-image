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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magiconair/properties"
	"github.com/mattn/go-shellwords"
	"github.com/paketo-buildpacks/libpak"
)

type Arguments interface {
	Configure(inputArgs []string) ([]string, string, error)
}

// BaselineArguments provides a set of arguments that are always set
type BaselineArguments struct {
	StackID string
}

// Configure provides an initial set of arguments, it ignores any input arguments
func (b BaselineArguments) Configure(_ []string) ([]string, string, error) {
	var newArguments []string

	if b.StackID == libpak.TinyStackID {
		newArguments = append(newArguments, "-H:+StaticExecutableWithDynamicLibC")
	}

	return newArguments, "", nil
}

// UserArguments augments the existing arguments with those provided by the end user
type UserArguments struct {
	Arguments string
}

// Configure returns the inputArgs plus the additional arguments specified by the end user, preference given to user arguments
func (u UserArguments) Configure(inputArgs []string) ([]string, string, error) {
	parsedArgs, err := shellwords.Parse(u.Arguments)
	if err != nil {
		return []string{}, "", fmt.Errorf("unable to parse arguments from %s\n%w", u.Arguments, err)
	}

	var outputArgs []string

	for _, inputArg := range inputArgs {
		if !containsArg(inputArg, parsedArgs) {
			outputArgs = append(outputArgs, inputArg)
		}
	}

	outputArgs = append(outputArgs, parsedArgs...)

	return outputArgs, "", nil
}

// UserFileArguments augments the existing arguments with those provided by the end user through a file
type UserFileArguments struct {
	ArgumentsFile string
}

// Configure returns the inputArgs plus the additional arguments specified by the end user through the file, preference given to user arguments
func (u UserFileArguments) Configure(inputArgs []string) ([]string, string, error) {
	rawArgs, err := ioutil.ReadFile(u.ArgumentsFile)
	if err != nil {
		return []string{}, "", fmt.Errorf("read arguments from %s\n%w", u.ArgumentsFile, err)
	}

	parsedArgs, err := shellwords.Parse(string(rawArgs))
	if err != nil {
		return []string{}, "", fmt.Errorf("unable to parse arguments from %s\n%w", string(rawArgs), err)
	}

	var outputArgs []string

	for _, inputArg := range inputArgs {
		if !containsArg(inputArg, parsedArgs) {
			outputArgs = append(outputArgs, inputArg)
		}
	}

	outputArgs = append(outputArgs, parsedArgs...)

	return outputArgs, "", nil
}

// containsArg checks if needle is found in haystack
//
// needle and haystack entries are processed as key=val strings where only the key must match
func containsArg(needle string, haystack []string) bool {
	needleSplit := strings.SplitN(needle, "=", 2)

	for _, straw := range haystack {
		targetSplit := strings.SplitN(straw, "=", 2)
		if needleSplit[0] == targetSplit[0] {
			return true
		}
	}

	return false
}

// ExplodedJarArguments provides a set of arguments specific to building from an exploded jar directory
type ExplodedJarArguments struct {
	ApplicationPath string
	LayerPath       string
	Manifest        *properties.Properties
}

// NoStartOrMainClass is an error returned when a start or main class cannot be found
type NoStartOrMainClass struct{}

func (e NoStartOrMainClass) Error() string {
	return "unable to read Start-Class or Main-Class from MANIFEST.MF"
}

// Configure appends arguments to inputArgs for building from an exploded JAR directory
func (e ExplodedJarArguments) Configure(inputArgs []string) ([]string, string, error) {
	startClass, ok := e.Manifest.Get("Start-Class")
	if !ok {
		startClass, ok = e.Manifest.Get("Main-Class")
		if !ok {
			return []string{}, "", NoStartOrMainClass{}
		}
	}

	cp := os.Getenv("CLASSPATH")
	if cp == "" {
		// CLASSPATH should have been done by upstream buildpacks, but just in case
		cp = e.ApplicationPath
		if v, ok := e.Manifest.Get("Class-Path"); ok {
			cp = strings.Join([]string{cp, v}, string(filepath.ListSeparator))
		}
	}

	inputArgs = append(inputArgs,
		fmt.Sprintf("-H:Name=%s", filepath.Join(e.LayerPath, startClass)),
		"-cp", cp,
		startClass,
	)

	return inputArgs, startClass, nil
}

// JarArguments provides a set of arguments specific to building from a jar file
type JarArguments struct {
	ApplicationPath string
	JarFilePattern  string
}

func (j JarArguments) Configure(inputArgs []string) ([]string, string, error) {
	file := filepath.Join(j.ApplicationPath, j.JarFilePattern)
	candidates, err := filepath.Glob(file)
	if err != nil {
		return []string{}, "", fmt.Errorf("unable to find JAR with %s\n%w", j.JarFilePattern, err)
	}

	if len(candidates) != 1 {
		sort.Strings(candidates)
		return []string{}, "", fmt.Errorf("unable to find single JAR in %s, candidates: %s", j.JarFilePattern, candidates)
	}

	jarFileName := filepath.Base(candidates[0])
	startClass := strings.TrimSuffix(jarFileName, ".jar")

	if containsArg("-jar", inputArgs) {
		var tmpArgs []string
		var skip bool
		for _, inputArg := range inputArgs {
			if skip {
				skip = false
				break
			}

			if inputArg == "-jar" {
				skip = true
				break
			}

			tmpArgs = append(tmpArgs, inputArg)
		}
		inputArgs = tmpArgs
	}

	inputArgs = append(inputArgs, "-jar", candidates[0])

	return inputArgs, startClass, nil
}
