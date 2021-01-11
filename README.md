# `gcr.io/paketo-buildpacks/spring-boot-native-image`
The Paketo Spring Boot Native Image Buildpack is a Cloud Native Buildpack that creates native images from Spring Boot applications.

## Behavior
This buildpack will participate all the following conditions are met

* `$BP_BOOT_NATIVE_IMAGE` is set

The buildpack will do the following:

* Creates a GraalVM native image and removes existing bytecode.

This buildpack requires that [Spring Native](https://github.com/spring-projects-experimental/spring-native) is included as an application dependency.

## Configuration
| Environment Variable | Description
| -------------------- | -----------
| `$BP_BOOT_NATIVE_IMAGE` | Whether to build a native image from the application.  Defaults to false.
| `$BP_BOOT_NATIVE_IMAGE_BUILD_ARGUMENTS` | Configure the arguments to pass to native image build

## Bindings
The buildpack optionally accepts the following bindings:

### Type: `dependency-mapping`
|Key                   | Value   | Description
|----------------------|---------|------------
|`<dependency-digest>` | `<uri>` | If needed, the buildpack will fetch the dependency with digest `<dependency-digest>` from `<uri>`

## License
This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0
[s]: https://github.com/spring-projects-experimental/spring-native
