import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "__ID__"
    compileSdk = 35

    defaultConfig {
        applicationId = "__ID__"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.getByName("debug") // sign with your own key before publishing
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    // What nimbusGo writes: the Go library per ABI and the generated Kotlin.
    sourceSets["main"].jniLibs.srcDir(layout.buildDirectory.dir("generated/nimbus/jniLibs"))
    sourceSets["main"].kotlin.srcDir(layout.buildDirectory.dir("generated/nimbus/kotlin"))
}

kotlin {
    compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
}

dependencies {
    implementation(project(":nimbusnativekit"))
}

// nimbusGo builds the app's Go code (app/mobile) into libnimbusapp.so and
// writes the Kotlin it needs, with the nimbus-native version in go.mod.
val nimbusGo by tasks.registering(Exec::class) {
    group = "nimbus"
    description = "Builds the app's Go code with nimbus native lib."
    val module = rootDir.resolve("../..").canonicalFile
    val out = layout.buildDirectory.dir("generated/nimbus").get().asFile
    val tool = layout.buildDirectory.file("nimbus/nimbus-native").get().asFile
    val kit = rootDir.resolve("nimbusnativekit/src/main/kotlin")
    val abis = nimbusABIs()

    inputs.files(fileTree(module) {
        include("**/*.go", "go.mod", "go.sum")
        exclude("native/**", "build/**")
    })
    inputs.property("abis", abis)
    outputs.dir(out)

    workingDir = module
    environment("PATH", goPath())
    environment("ANDROID_HOME", android.sdkDirectory.path)
    commandLine("sh", "-c", """
        set -e
        CGO_ENABLED=0 go build -o "$tool" github.com/CodeSyncr/nimbus-native/cmd/nimbus-native
        exec "$tool" lib android -id ${android.namespace} -abi $abis -jni "$out/jniLibs" -kotlin "$out/kotlin" -kit "$kit"
    """.trimIndent())
}
tasks.named("preBuild") { dependsOn(nimbusGo) }

// The ABIs to build: just the device's when Android Studio deploys to one,
// otherwise every ABI a release ships.
fun nimbusABIs(): String {
    val known = listOf("arm64-v8a", "armeabi-v7a", "x86_64", "x86")
    val device = (findProperty("android.injected.build.abi") as String?)
        ?.split(",")?.firstOrNull { it in known }
    return device ?: "arm64-v8a,armeabi-v7a,x86_64"
}

// Android Studio started from the Dock doesn't see the shell's PATH.
fun goPath(): String {
    val home = System.getProperty("user.home")
    return listOf("$home/go/bin", "$home/sdk/go/bin", "/usr/local/go/bin", "/opt/homebrew/bin", "/usr/local/bin",
        System.getenv("PATH") ?: "").joinToString(":")
}
