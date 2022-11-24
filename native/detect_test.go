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
	"os"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/native-image/v5/native"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.DetectContext
		detect native.Detect
	)

	context("neither BP_NATIVE_IMAGE nor BP_BOOT_NATIVE_IMAGE are set", func() {
		it("provides but does not requires native-image-application", func() {
			Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "jvm-application",
								Metadata: map[string]interface{}{"native-image": true},
							},
							{
								Name:     "spring-boot",
								Metadata: map[string]interface{}{"native-image": true},
							},
						},
					},
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "native-image-argfile",
							},
							{
								Name: "native-image-application",
							},
						},
					},
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "jvm-application",
								Metadata: map[string]interface{}{"native-image": true},
							},
						},
					},
				},
			}))
		})
	})

	context("$BP_NATIVE_IMAGE", func() {
		context("true", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_NATIVE_IMAGE", "true")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_NATIVE_IMAGE")).To(Succeed())
			})

			it("provides and requires native-image-application", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
					},
				}))
			})
		})

		context("false", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_NATIVE_IMAGE", "false")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_NATIVE_IMAGE")).To(Succeed())
			})

			it("provides but does not requires native-image-application", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
							},
						},
					},
				}))
			})
		})



		context("not a bool", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_NATIVE_IMAGE", "foo")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_NATIVE_IMAGE")).To(Succeed())
			})

			it("errors", func() {
				_, err := detect.Detect(ctx)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	context("$BP_BINARY_COMPRESSION_METHOD", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_NATIVE_IMAGE", "true")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_NATIVE_IMAGE")).To(Succeed())
		})

		context("upx", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_BINARY_COMPRESSION_METHOD", "upx")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_BINARY_COMPRESSION_METHOD")).To(Succeed())
			})

			it("requires upx", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
								{
									Name: "upx",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
								{
									Name: "upx",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
								{
									Name: "upx",
								},
							},
						},
					},
				}))
			})
		})

		context("gzexe", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_BINARY_COMPRESSION_METHOD", "gzexe")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_BINARY_COMPRESSION_METHOD")).To(Succeed())
			})

			it("no additional provides or requires", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
					},
				}))
			})
		})

		context("none", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_BINARY_COMPRESSION_METHOD", "none")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_BINARY_COMPRESSION_METHOD")).To(Succeed())
			})

			it("no additional provides or requires", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
					},
				}))
			})
		})

		context("not a supported method", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_BINARY_COMPRESSION_METHOD", "foo")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_BINARY_COMPRESSION_METHOD")).To(Succeed())
			})

			it("ignore and no additional provides or requires", func() {
				Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
					Pass: true,
					Plans: []libcnb.BuildPlan{
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name:     "spring-boot",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "native-image-argfile",
								},
								{
									Name: "native-image-application",
								},
							},
						},
						{
							Provides: []libcnb.BuildPlanProvide{
								{Name: "native-image-application"},
							},
							Requires: []libcnb.BuildPlanRequire{
								{
									Name: "native-image-builder",
								},
								{
									Name:     "jvm-application",
									Metadata: map[string]interface{}{"native-image": true},
								},
								{
									Name: "native-image-application",
								},
							},
						},
					},
				}))
			})
		})
	})

	context("$BP_BOOT_NATIVE_IMAGE", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_BOOT_NATIVE_IMAGE", "true")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_BOOT_NATIVE_IMAGE")).To(Succeed())
		})

		it("provides and requires native-image-application", func() {
			Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "jvm-application",
								Metadata: map[string]interface{}{"native-image": true},
							},
							{
								Name:     "spring-boot",
								Metadata: map[string]interface{}{"native-image": true},
							},
							{
								Name: "native-image-application",
							},
						},
					},
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "native-image-argfile",
							},
							{
								Name: "native-image-application",
							},
						},
					},
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: "native-image-application"},
						},
						Requires: []libcnb.BuildPlanRequire{
							{
								Name: "native-image-builder",
							},
							{
								Name:     "jvm-application",
								Metadata: map[string]interface{}{"native-image": true},
							},
							{
								Name: "native-image-application",
							},
						},
					},
				},
			}))
		})
	})
}
