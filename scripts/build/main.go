// Command build cross-compiles the lastab CLI into resources/<platform>/bin,
// where mygo build bundles and signs it with the app, then runs mygo build.
//
//	go run ./scripts/build -cli-only                          # this machine's CLI only
//	go run ./scripts/build                                    # this machine's app
//	go run ./scripts/build -platform darwin/universal          # .app + DMG
//	go run ./scripts/build -platform windows/amd64,windows/arm64
//
// Signing comes from the environment, so that certificates stay in CI
// secrets and keychains and out of mygo.json. Each variable that is set adds
// its setting to mygo.json for the length of the build; without them the
// macOS app is signed ad hoc and the Windows apps are not signed.
//
//	MACOS_SIGNING_IDENTITY      macos.signingIdentity, e.g. "Developer ID Application: … (TEAMID)"
//	NOTARY_KEYCHAIN_PROFILE     macos.notarize.keychainProfile (xcrun notarytool store-credentials)
//	NOTARY_KEYCHAIN             macos.notarize.keychain, the keychain holding the profile
//	WINDOWS_CERTIFICATE_FILE    windows.certificate, a .pfx; its password in MYGO_WINDOWS_CERTIFICATE_PASSWORD
//	WINDOWS_SIGN_COMMAND        windows.signCommand instead, with %1 for the file
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const configFile = "mygo.json"

func main() {
	platform := flag.String("platform", runtime.GOOS+"/"+runtime.GOARCH, "mygo build platforms, comma-separated")
	cliOnly := flag.Bool("cli-only", false, "build the CLI only")
	flag.Parse()

	orig, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	version := appVersion(orig)
	var targets [][2]string
	for _, p := range strings.Split(*platform, ",") {
		goos, arch, _ := strings.Cut(strings.TrimSpace(p), "/")
		if arch == "universal" {
			targets = append(targets, [2]string{goos, "arm64"}, [2]string{goos, "amd64"})
			continue
		}
		targets = append(targets, [2]string{goos, arch})
	}
	for _, t := range targets {
		exe := "lastab"
		if t[0] == "windows" {
			exe += ".exe"
		}
		out := filepath.Join("resources", t[0]+"-"+t[1], "bin", exe)
		if err := run(append(os.Environ(), "GOOS="+t[0], "GOARCH="+t[1], "CGO_ENABLED=0"),
			"go", "build", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", out, "./cmd/lastab"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("built", out)
	}
	if *cliOnly {
		return
	}

	signed, notes, err := withSigning(orig, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	for _, n := range notes {
		for _, t := range targets {
			if strings.HasPrefix(n, t[0]+":") {
				fmt.Println(n)
				break
			}
		}
	}
	restore := func() {}
	if !bytes.Equal(signed, orig) {
		if err := os.WriteFile(configFile, signed, 0o644); err != nil {
			log.Fatal(err)
		}
		restore = func() {
			if err := os.WriteFile(configFile, orig, 0o644); err != nil {
				log.Printf("restore %s: %v", configFile, err)
			}
		}
		// Put mygo.json back however the build ends, Ctrl+C included.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sig
			restore()
			os.Exit(130)
		}()
	}
	env := append(os.Environ(), "CGO_ENABLED=0")
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		// Homebrew's makensis aborts (std::bad_alloc) without a locale.
		env = append(env, "LANG=C.UTF-8")
	}
	err = run(env, "go", "tool", "mygo", "build", "-platform", *platform)
	restore()
	if err != nil {
		log.Fatal(err)
	}
}

// withSigning returns mygo.json with the signing settings the environment
// gives, and notes on what is and isn't signed.
func withSigning(config []byte, getenv func(string) string) ([]byte, []string, error) {
	var cfg map[string]any
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", configFile, err)
	}
	section := func(name string) map[string]any {
		m, _ := cfg[name].(map[string]any)
		if m == nil {
			m = map[string]any{}
			cfg[name] = m
		}
		return m
	}
	var notes []string
	changed := false
	if id := getenv("MACOS_SIGNING_IDENTITY"); id != "" {
		section("macos")["signingIdentity"] = id
		changed = true
		notes = append(notes, "darwin: signing with "+id)
		if profile := getenv("NOTARY_KEYCHAIN_PROFILE"); profile != "" {
			n := map[string]any{"keychainProfile": profile}
			if kc := getenv("NOTARY_KEYCHAIN"); kc != "" {
				n["keychain"] = kc
			}
			section("macos")["notarize"] = n
			notes = append(notes, "darwin: notarizing with the keychain profile "+profile)
		} else {
			notes = append(notes, "darwin: not notarizing (NOTARY_KEYCHAIN_PROFILE is not set)")
		}
	} else {
		notes = append(notes, "darwin: signing ad hoc (MACOS_SIGNING_IDENTITY is not set)")
	}
	switch cert, cmd := getenv("WINDOWS_CERTIFICATE_FILE"), getenv("WINDOWS_SIGN_COMMAND"); {
	case cert != "":
		section("windows")["certificate"] = cert
		changed = true
		notes = append(notes, "windows: signing with "+filepath.Base(cert))
	case cmd != "":
		section("windows")["signCommand"] = cmd
		changed = true
		notes = append(notes, "windows: signing with WINDOWS_SIGN_COMMAND")
	default:
		notes = append(notes, "windows: not signing (WINDOWS_CERTIFICATE_FILE is not set)")
	}
	if !changed {
		return config, notes, nil
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return append(out, '\n'), notes, nil
}

func appVersion(config []byte) string {
	var cfg struct{ Version string }
	if err := json.Unmarshal(config, &cfg); err != nil {
		log.Fatalf("%s: %v", configFile, err)
	}
	return cfg.Version
}

func run(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr = env, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return nil
}
