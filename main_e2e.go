//go:build e2e

package main

// Lets mygo-automation drive a build made with -tags e2e, when
// MYGO_AUTOMATION names a socket. Release builds leave it out.
import _ "github.com/egoist/mygo/plugins/automation"
