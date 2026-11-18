package shell

import "time"

// nowFrame is the time for actions started from the menu, outside a frame.
func nowFrame() time.Time { return time.Now() }
