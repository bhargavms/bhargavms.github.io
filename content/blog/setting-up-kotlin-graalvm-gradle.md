---
title: "Setting Up a Production-Grade Kotlin Project with GraalVM and Gradle"
date: 2025-01-11T13:30:02+07:00
description: "A walkthrough for a production-grade Kotlin project with GraalVM native image support, version catalogs, and included build-logic."
tags:
  - kotlin
  - gradle
  - graalvm
draft: false
---

Modern Kotlin projects require a robust build setup that's maintainable, scalable, and follows best practices. This guide will walk you through setting up a production-grade Kotlin project with GraalVM support using Gradle's latest features including version catalogs and included builds.

## Project Structure

First, let's look at our project structure:

```
├── build-logic/
│   ├── settings.gradle.kts
│   ├── build.gradle.kts
│   └── src/main/kotlin/
│       └── your/
│           └── gradle/
│               ├── kotlin-common.gradle.kts
│               └── graalvm.gradle.kts
├── gradle/
│   └── libs.versions.toml
├── settings.gradle.kts
└── build.gradle.kts
```

## Version Catalog

The version catalog (`gradle/libs.versions.toml`) centralizes all our dependency versions and plugin declarations. This makes version management easier and ensures consistency across modules.

```toml
[versions]
kotlin = "1.9.22"
graalvm = "0.9.28"
coroutines = "1.7.3"
junit-jupiter = "5.10.1"
assertk = "0.27.0"
slf4j = "2.0.9"
logback = "1.4.14"

[libraries]
kotlin-stdlib = { module = "org.jetbrains.kotlin:kotlin-stdlib", version.ref = "kotlin" }
kotlin-coroutines = { module = "org.jetbrains.kotlinx:kotlinx-coroutines-core", version.ref = "coroutines" }
kotlin-test = { module = "org.jetbrains.kotlin:kotlin-test", version.ref = "kotlin" }
junit-jupiter = { module = "org.junit.jupiter:junit-jupiter", version.ref = "junit-jupiter" }
assertk = { module = "com.willowtreeapps.assertk:assertk", version.ref = "assertk" }
slf4j-api = { module = "org.slf4j:slf4j-api", version.ref = "slf4j" }
logback-classic = { module = "ch.qos.logback:logback-classic", version.ref = "logback" }

[plugins]
kotlin = { id = "org.jetbrains.kotlin.jvm", version.ref = "kotlin" }
graalvm = { id = "org.graalvm.buildtools.native", version.ref = "graalvm" }
```

## Root Settings

The root `settings.gradle.kts` configures dependency resolution and includes our build logic:

```kotlin
// settings.gradle.kts
rootProject.name = "your-project-name"

pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        google()
    }
}

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
    versionCatalogs {
        create("libs") {
            from(files("gradle/libs.versions.toml"))
        }
    }
}

includeBuild("build-logic")
```

## Build Logic Setup

The build logic module contains our convention plugins. First, its settings file:

```kotlin
// build-logic/settings.gradle.kts
dependencyResolutionManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
    versionCatalogs {
        create("libs") {
            from(files("../gradle/libs.versions.toml"))
        }
    }
}

rootProject.name = "build-logic"
```

And its build file:

```kotlin
// build-logic/build.gradle.kts
plugins {
    `kotlin-dsl`
}

dependencies {
    implementation(libs.plugins.kotlin.get().pluginId)
    implementation(libs.plugins.graalvm.get().pluginId)
}
```

## Convention Plugins

These plugins encapsulate our build logic in a reusable way. Here's our Kotlin convention:

```kotlin
// build-logic/src/main/kotlin/your/gradle/kotlin-common.gradle.kts
package your.gradle

plugins {
    id("org.jetbrains.kotlin.jvm")
}

kotlin {
    jvmToolchain(17)
    explicitApi()
}

tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile>().configureEach {
    kotlinOptions {
        jvmTarget = "17"
        allWarningsAsErrors = true
        freeCompilerArgs = freeCompilerArgs + listOf(
            "-Xjsr305=strict",
            "-opt-in=kotlin.RequiresOptIn"
        )
    }
}

tasks.withType<Test>().configureEach {
    useJUnitPlatform()
    maxParallelForks = (Runtime.getRuntime().availableProcessors() / 2).takeIf { it > 0 } ?: 1
}
```

And our GraalVM convention:

```kotlin
// build-logic/src/main/kotlin/your/gradle/graalvm.gradle.kts
package your.gradle

plugins {
    id("org.graalvm.buildtools.native")
}

graalvmNative {
    binaries {
        named("main") {
            imageName.set(project.name)
            mainClass.set("com.example.MainKt") // Customize this
            debug.set(System.getProperty("debug") != null)
            buildArgs.addAll(
                "--no-fallback",
                "-H:+ReportExceptionStackTraces",
                "--enable-url-protocols=http,https"
            )
            verbose.set(true)
        }
    }
    metadataRepository {
        enabled.set(true)
    }
}
```

## Main Build File

Our root build file brings everything together:

```kotlin
// build.gradle.kts
plugins {
    alias(libs.plugins.kotlin)
    alias(libs.plugins.graalvm)
    id("your.gradle.kotlin-common")
    id("your.gradle.graalvm")
    application
}

group = "com.example"
version = "0.1.0-SNAPSHOT"

dependencies {
    implementation(libs.kotlin.stdlib)
    implementation(libs.kotlin.coroutines)
    implementation(libs.slf4j.api)
    implementation(libs.logback.classic)

    testImplementation(libs.kotlin.test)
    testImplementation(libs.junit.jupiter)
    testImplementation(libs.assertk)
}

application {
    mainClass.set("com.example.MainKt")
}
```

## Additional Configuration Files

### Git Ignore
```gitignore
.gradle/
build/
out/
*.iml
.idea/
.DS_Store
```

### Gradle Properties
```properties
org.gradle.parallel=true
org.gradle.caching=true
org.gradle.configuration-cache=true
kotlin.code.style=official
kotlin.incremental=true
```

## Usage

Build the project:
```bash
./gradlew build
```

Run tests:
```bash
./gradlew test
```

Create native image:
```bash
./gradlew nativeCompile
```

## Benefits of This Setup

1. **Centralized Version Management**: All versions are managed in a single `libs.versions.toml` file
2. **Reusable Build Logic**: Convention plugins keep build logic DRY and maintainable
3. **Type-Safe Build Scripts**: Using Kotlin DSL provides better IDE support and compile-time checks
4. **Optimized Build Performance**: Parallel execution, build caching, and configuration caching enabled
5. **Production-Ready**: Includes logging, testing, and native image support out of the box
6. **Strong Kotlin Configuration**: Strict compiler settings and explicit API mode enabled

## Next Steps

You can extend this setup by:
- Adding code coverage reporting with Kover
- Configuring static analysis tools
- Setting up CI/CD pipelines
- Adding documentation generation
- Configuring artifact publishing


Remember to customize the main class path and other project-specific settings before using this template.
