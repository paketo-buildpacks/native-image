# `gcr.io/paketo-buildpacks/native-image`
The Paketo Native Image Buildpack is a Cloud Native Buildpack that uses the [GraalVM Native Image builder][native-image] (`native-image`) to compile a standalone executable from an executable JAR.

Most users should not use this component buildpack directly and should instead use the [Paketo Java Native Image][bp/java-native-image], which provides the full set of buildpacks required to build a native image application.

## Behavior
This buildpack will participate if one the following conditions are met:

* `$BP_NATIVE_IMAGE` is set.
*  An upstream buildpack requests `native-image-application` in the build plan.

The buildpack will do the following:

* Requests that the Native Image builder be installed by requiring `native-image-builder` in the build plan.
* Uses `native-image` a to build a GraalVM native image and removes existing bytecode.

## Configuration
| Environment Variable | Description
| -------------------- | -----------
| `$BP_NATIVE_IMAGE` | Whether to build a native image from the application.  Defaults to false.
| `$BP_NATIVE_IMAGE_BUILD_ARGUMENTS` | Arguments to pass to the `native-image` command.

## License
This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0
[native-image]: https://www.graalvm.org/reference-manual/native-image/
[bp/java-native-image]: https://github.com/paketo-buildpacks/java-native-image
