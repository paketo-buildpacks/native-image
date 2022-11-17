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
	"strconv"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
)

const (
	ConfigNativeImage           = "BP_NATIVE_IMAGE"
	DeprecatedConfigNativeImage = "BP_BOOT_NATIVE_IMAGE"
	BinaryCompressionMethod     = "BP_BINARY_COMPRESSION_METHOD"

	PlanEntryNativeImage        = "native-image-application"
	PlanEntryNativeImageBuilder = "native-image-builder"
	PlanEntryJVMApplication     = "jvm-application"
	PlanEntrySpringBoot    		= "spring-boot"
	PlanEntryNativeArgFile		= "native-image-argfile"
	PlanEntryUpx       		    = "upx"
)

type Detect struct{}

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	cr, err := libpak.NewConfigurationResolver(context.Buildpack, nil)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{
						Name: PlanEntryNativeImage,
					},
				},
				Requires: []libcnb.BuildPlanRequire{
					{
						Name: PlanEntryNativeImageBuilder,
					},
					{
						Name:     PlanEntryJVMApplication,
						Metadata: map[string]interface{}{"native-image": true},
					},
					{
						Name:     PlanEntrySpringBoot,
						Metadata: map[string]interface{}{"native-image": true},
					},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{
						Name: PlanEntryNativeImage,
					},
				},
				Requires: []libcnb.BuildPlanRequire{
					{
						Name: PlanEntryNativeImageBuilder,
					},
					{
						Name: PlanEntryNativeArgFile,
					},
					{
						Name: PlanEntryNativeImage,
					},
				},
			},
			{
				Provides: []libcnb.BuildPlanProvide{
					{
						Name: PlanEntryNativeImage,
					},
				},
				Requires: []libcnb.BuildPlanRequire{
					{
						Name: PlanEntryNativeImageBuilder,
					},
					{
						Name:     PlanEntryJVMApplication,
						Metadata: map[string]interface{}{"native-image": true},
					},
				},
			},
		},
	}

	if ok, err := d.nativeImageEnabled(cr); err != nil {
		return libcnb.DetectResult{}, err
	} else if ok {
		for i := range result.Plans {
			found := false
			for _, r := range result.Plans[i].Requires {
				if r.Name == PlanEntryNativeImage{
					found = true
				}
			}
			if !found {
				result.Plans[i].Requires = append(result.Plans[i].Requires, libcnb.BuildPlanRequire{
					Name: PlanEntryNativeImage,
				})
			}
		}
	}

	if d.upxCompressionEnabled(cr) {
		for i := range result.Plans {
			result.Plans[i].Requires = append(result.Plans[i].Requires, libcnb.BuildPlanRequire{
				Name: PlanEntryUpx,
			})
		}
	}

	// still participates if a downstream buildpack requires native-image-applications or upx
	return result, nil
}

func (d Detect) upxCompressionEnabled(cr libpak.ConfigurationResolver) bool {
	if val, ok := cr.Resolve(BinaryCompressionMethod); ok {
		return val == CompressorUpx
	}
	return false
}

func (d Detect) nativeImageEnabled(cr libpak.ConfigurationResolver) (bool, error) {
	if val, ok := cr.Resolve(ConfigNativeImage); ok {
		enable, err := strconv.ParseBool(val)
		if err != nil {
			return false, fmt.Errorf(
				"invalid value '%s' for key '%s': expected one of [1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False]",
				val,
				ConfigNativeImage,
			)
		}
		return enable, nil
	}
	_, ok := cr.Resolve(DeprecatedConfigNativeImage)
	return ok, nil
}
