# Testowanie BP_NATIVE_IMAGE_INCLUDE_FILES lokalnie

## Wymagania

- Docker
- `pack` CLI (v0.40.0+)
- Go (do zbudowania buildpacka)

## Krok 1: Zainstaluj narzedzia

```bash
# pack CLI
curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.40.0/pack-v0.40.0-linux.tgz" | sudo tar -C /usr/local/bin -xz pack

# create-package (narzedzie paketo do budowania buildpackow)
go install github.com/paketo-buildpacks/libpak/cmd/create-package@latest
```

## Krok 2: Spakuj zmodyfikowany buildpack jako obraz Docker

```bash
cd /sciezka/do/native-image

# Zbuduj binarki (skrypt pre-package)
./scripts/build.sh

# Stworz paczke buildpacka
create-package --source . --destination ./buildpack-out --version 0.0.0-dev

# Spakuj jako obraz Docker
pack config experimental true
pack buildpack package my-native-image:latest --format image --path ./buildpack-out
```

## Krok 3: Stworz minimalna aplikacje Spring Boot

```bash
mkdir -p /tmp/test-include-files && cd /tmp/test-include-files
```

### pom.xml

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0
         https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.2.5</version>
    </parent>
    <groupId>com.example</groupId>
    <artifactId>demo</artifactId>
    <version>0.0.1</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
    </dependencies>
</project>
```

### src/main/java/com/example/demo/DemoApplication.java

```java
package com.example.demo;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.*;

@SpringBootApplication
@RestController
public class DemoApplication {
    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }

    @GetMapping("/")
    public String hello() { return "Hello Native!"; }
}
```

## Krok 4: Stworz falszywy folder Dynatrace

Ten folder symuluje to, co Dynatrace tworzy w trakcie budowania native image.

Buildpack obsluguje dwa scenariusze:

**Scenariusz A** -- folder na top-level (exploded JAR / Spring Boot):
```bash
cd /tmp/test-include-files
mkdir -p dynatrace/agent/conf
echo '{"agentId":"test-123"}' > dynatrace/agent/conf/config.json
```

**Scenariusz B** -- folder zagniezdony w target/ (build z JAR):
```bash
cd /tmp/test-include-files
mkdir -p target/dynatrace/agent/conf
echo '{"agentId":"test-123"}' > target/dynatrace/agent/conf/config.json
```

W scenariuszu B folder `target/dynatrace` zostanie wyekstrahowany i umieszczony
jako `dynatrace` w roocie obrazu (obok pliku wykonywalnego).

## Krok 5: Zbuduj obraz z pack build

### Scenariusz A: folder na top-level

```bash
cd /tmp/test-include-files

pack build test-native-app:latest \
  --builder harbor.p4.int/registrydockerio/paketobuildpacks/builder-jammy-full:0.3.595 \
  --path /home/maculem/projects/sample-springboot21 \
  --buildpack paketo-buildpacks/ca-certificates@3.12.0 \
  --buildpack paketo-buildpacks/bellsoft-liberica@11.6.0 \
  --buildpack paketo-buildpacks/syft@2.31.0 \
  --buildpack paketo-buildpacks/maven@6.22.0 \
  --buildpack paketo-buildpacks/executable-jar@6.15.0 \
  --buildpack paketo-buildpacks/spring-boot@5.36.0 \
  --buildpack my-native-image:latest \
  --env BP_NATIVE_IMAGE=true \
  --env BP_INCLUDE_FILES="target/tmp:target/dynatrace" \
  --env BP_NATIVE_IMAGE_INCLUDE_FILES="target/dynatrace" \
  --env BP_NATIVE_IMAGE_BUILD_ARGUMENTS="-J-agentpath:/home/maculem/projects/sample-springboot21/dt-agent/graalnative/buildtime/lib64/liboneagentgraalnativebuildtime.so=loglevelcon=debug -cp /home/maculem/projects/sample-springboot21/dt-agent/graalnative/buildtime/any/java/oneagent-graalnative-feature.jar --features=com.dynatrace.graalnative.features.OneAgentGraalNativeFeature --enable-url-protocols=http,https" \
  --verbose
```

W logach powinienes zobaczyc:
```
Paketo Buildpack for Native Image ...
  Removing bytecode
    Preserving dynatrace
```

### Scenariusz B: folder zagniezdony w target/

```bash
cd /tmp/test-include-files

pack build test-native-app:latest \
  --builder harbor.p4.int/registrydockerio/paketobuildpacks/builder-jammy-full:0.3.595 \
  --buildpack paketo-buildpacks/java-native-image \
  --buildpack my-native-image:latest \
  --env BP_NATIVE_IMAGE=true \
  --env BP_NATIVE_IMAGE_INCLUDE_FILES=target/dynatrace \
  --verbose
```

W logach powinienes zobaczyc:
```
Paketo Buildpack for Native Image ...
  Removing bytecode
    Saving target/dynatrace for inclusion
    ...
    Restoring dynatrace
```

Wzorzec `target/dynatrace` oznacza: znajdz `target/dynatrace`, usun caly bytecode
(w tym `target/`), a nastepnie umiesc `dynatrace` w roocie aplikacji obok binarki.

Co tu sie dzieje:
- `--builder paketobuildpacks/builder-jammy-tiny` -- builder z GraalVM
- `--buildpack paketo-buildpacks/java-native-image` -- composite buildpack
  z zaleznosci (Maven, GraalVM, Spring Boot itd.)
- `--buildpack my-native-image:latest` -- nasz zmodyfikowany buildpack,
  ktory nadpisuje ten z composite (pack uzyje ostatniego pasujacego po ID)
- `BP_NATIVE_IMAGE=true` -- wlacza budowanie native image
- `BP_NATIVE_IMAGE_INCLUDE_FILES` -- nasza nowa zmienna, obsluguje:
  - wzorce top-level: `dynatrace`, `*.conf`
  - wzorce zagniezdone: `target/dynatrace`, `target/dt-*`

## Krok 6: Zweryfikuj wynikowy obraz

```bash
# Sprawdz czy folder dynatrace przetrwal (oba scenariusze daja ten sam wynik)
docker run --rm -it --entrypoint /bin/sh test-native-app:latest -c \
  "ls -la /workspace/dynatrace/ && cat /workspace/dynatrace/agent/conf/config.json"
```

Oczekiwany wynik:
```
total ...
drwxr-xr-x  agent
{"agentId":"test-123"}
```

Uwaga: w scenariuszu B folder `target/` NIE powinien istniec w obrazie:
```bash
docker run --rm -it --entrypoint /bin/sh test-native-app:latest -c \
  "ls /workspace/target 2>&1 || echo 'target/ usuniety - OK'"
```

## Krok 7: Test negatywny (bez BP_NATIVE_IMAGE_INCLUDE_FILES)

```bash
pack build test-native-app-no-include:latest \
  --builder paketobuildpacks/builder-jammy-tiny \
  --buildpack paketo-buildpacks/java-native-image \
  --buildpack my-native-image:latest \
  --env BP_NATIVE_IMAGE=true

docker run --rm -it --entrypoint /bin/sh test-native-app-no-include:latest -c \
  "ls /workspace/dynatrace 2>&1 || echo 'FOLDER NIE ISTNIEJE - zostal usuniety'"
```

Oczekiwany wynik: `FOLDER NIE ISTNIEJE - zostal usuniety`

## Krok 8 (opcjonalnie): Test z mieszanymi wzorcami

```bash
# Przygotuj pliki
mkdir -p /tmp/test-include-files/target/dynatrace
echo "extra-config" > /tmp/test-include-files/monitoring.conf

pack build test-native-app-multi:latest \
  --builder paketobuildpacks/builder-jammy-tiny \
  --buildpack paketo-buildpacks/java-native-image \
  --buildpack my-native-image:latest \
  --env BP_NATIVE_IMAGE=true \
  --env "BP_NATIVE_IMAGE_INCLUDE_FILES=target/dynatrace *.conf"

docker run --rm -it --entrypoint /bin/sh test-native-app-multi:latest -c \
  "ls /workspace/dynatrace && cat /workspace/monitoring.conf"
```

## Rozwiazywanie problemow

| Problem | Rozwiazanie |
|---------|-------------|
| `pack buildpack package` nie dziala | Upewnij sie ze `pack config experimental true` zostalo wywolane |
| Builder nie ma GraalVM | Uzyj `paketobuildpacks/builder-jammy-tiny` lub `builder-jammy-full` |
| Native image build trwa dlugo | Normalne -- GraalVM potrzebuje min. 4-8 GB RAM i ~5 min |
| Nasz buildpack nie nadpisuje domyslnego | Sprawdz czy ID w buildpack.toml to `paketo-buildpacks/native-image` |
| `my-native-image:latest` nie istnieje | Upewnij sie ze krok 2 przeszedl bez bledow |
