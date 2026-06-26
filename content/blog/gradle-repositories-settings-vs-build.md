---
title: "Repositories in settings.gradle.kts vs build.gradle.kts"
date: 2025-01-11T12:47:12+07:00
description: "Understanding the difference between repository blocks in Gradle settings and build scripts, and how to enforce strict repository management."
tags:
  - gradle
  - kotlin
draft: false
---

1. `settings.gradle.kts` (Project Settings):

```kotlin
// settings.gradle.kts
dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}
```

* This configures repositories for ALL project dependencies
* Applied before any build scripts are evaluated
* Sets the default repositories for the entire project and all its modules
* Available since Gradle 6.8
* Considered the modern, recommended way to declare repositories

2. `build.gradle.kts` (BuildScript):

```kotlin
// build.gradle.kts
buildscript {
    repositories {
        mavenCentral()
    }
}
```
* This configures repositories only for build script dependencies (plugins, etc.)
* Only affects dependencies needed to execute the build script itself
* Doesn't affect project dependencies
* Used primarily for legacy plugins or when you need specific repositories just for build configuration

**Key Distinctions:**
* Scope: `settings.gradle.kts` affects the entire project, while buildscript block only affects build script dependencies
* Timing: Settings script runs first, then build scripts
* Purpose: Settings file configures project-wide behavior, build script configures build-specific behavior

**Best Practice:**

* Use dependencyResolutionManagement in settings.gradle.kts for project-wide repository configuration
* Only use buildscript repositories when you need specific repositories for build script dependencies that aren't available through the modern plugin system

Now that we understand what the differences are, and what is the purpose for repositories block inside the buildscript block of build.script.kts files, we can modernize this as well since settings.gradle.kts provides a way to declare repositories for the purpose of use inside build script execution scope.

### How to define repositories for the purpose of build script execution scope.

`settings.gradle.kts` has a special block called the `pluginManagement` block, inside of this we can define repositories for above purpose.

The repositories in buildscript and pluginManagement serve similar purposes but have some key differences. Let me explain and show you how to enforce strict repository management:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}
```
* Configures where Gradle looks for plugins specified in the plugins block
* Runs before anything else, even before buildscript blocks
* Is the modern, recommended way to manage plugin repositories

2. `buildscript` inside `build.gradle.kts`:

```kotlin
// build.gradle.kts
buildscript {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}
```
* Legacy way to configure plugin repositories
* Used when plugins are declared using the buildscript classpath
* Still needed for some legacy plugins that don't support the modern plugins DSL

As we can see from above this causes ambiguity in our build configuration, users/developers can define repository dependencies either in settings.gradle.kts file or in individual build.gradle.kts files of each module/project is not ideal, since in that we won't be able to deduce and alter repository dependencies easily.

**To enforce strict repository management and avoid ambiguity:**

1. Use Repository Rules:

```kotlin
// settings.gradle.kts
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
        google()
    }
}

pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}
```
2. Remove all repository declarations from build files:

```kotlin
// build.gradle.kts
// NO repositories {} blocks here
// NO buildscript {} repository blocks

plugins {
    // Use this modern syntax instead
    id("org.jetbrains.kotlin.jvm") version "1.9.22"
}
```

Benefits of this approach:

* Single source of truth for repositories
* Fail-fast if projects try to declare their own repositories
* Clear separation between plugin and dependency repositories
* Better security by explicitly controlling repository sources

If you still need to use a legacy plugin:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        // Add any legacy plugin repositories here
        maven {
            url = uri("https://legacy-plugin-repo.com")
        }
    }
}

// Then in build.gradle.kts, still use buildscript if absolutely necessary
buildscript {
    dependencies {
        classpath("legacy:plugin:1.0.0")
    }
}
```
The key is to:

* Use RepositoriesMode.FAIL_ON_PROJECT_REPOS
* Define all repositories in settings.gradle.kts
* Use the modern plugins DSL whenever possible
* Only use buildscript for legacy plugins that absolutely require it
