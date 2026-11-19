// Package fonts embeds the interface and editor fonts, Inter and
// JetBrains Mono (both SIL Open Font License 1.1, see the license files
// beside them), and registers them with MyGo. Chinese falls back to the
// system's PingFang SC or Microsoft YaHei. mygo.json leaves this folder
// out of the app's resources: the fonts are in the binary.
package fonts

import (
	"embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Families as views name them.
const (
	UI   = "Inter"
	Mono = "JetBrains Mono"
)

//go:embed *.ttf
var files embed.FS

var (
	once sync.Once
	err  error
)

// Register adds the fonts under their families, once: Inter's Regular,
// Medium and SemiBold make up one family, matched by weight, as do
// JetBrains Mono's Regular and Medium.
func Register() error {
	once.Do(func() {
		for _, f := range []struct{ file, family string }{
			{"Inter-Regular.ttf", UI},
			{"Inter-Medium.ttf", UI},
			{"Inter-SemiBold.ttf", UI},
			{"JetBrainsMono-Regular.ttf", Mono},
			{"JetBrainsMono-Medium.ttf", Mono},
		} {
			b, rerr := files.ReadFile(f.file)
			if rerr == nil {
				rerr = ui.RegisterFont(b, f.family)
			}
			if rerr != nil {
				err = fmt.Errorf("font %s: %w", f.file, rerr)
				return
			}
		}
	})
	return err
}
