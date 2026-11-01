/// lightsensor: iOS doesn't let apps read this sensor.
enum LightSensorPlugin {
    static func handle(_ call: PluginCall) {
        if call.method == "available" { return call.resolve(["available": false]) }
        call.reject(Plugins.unavailable, "iOS doesn't let apps read the light sensor")
    }
}
