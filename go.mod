module github.com/CodeSyncr/nimbus-native

go 1.23.3

// v0.0.1: iOS projects made from the module proxy lack nn.h (module zips
// leave symlinks out). Use v0.0.2.
retract v0.0.1
