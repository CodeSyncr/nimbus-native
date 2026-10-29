import org.jetbrains.kotlin.gradle.dsl.JvmTarget

// The Android host kit. Its sources (src/main/kotlin) are copied from the
// nimbus-native version in go.mod by the app's nimbusGo task: do not edit them.
plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "space.nimbusgo.nativekit"
    compileSdk = 35

    defaultConfig {
        minSdk = 26
        consumerProguardFiles("consumer-rules.pro")
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
}
