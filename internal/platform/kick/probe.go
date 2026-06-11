package kick

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/aalejandrofer/grubdrops/internal/platform"
)

// Probe is a one-shot diagnostic that hits the live Kick endpoints with a real
// authed session and dumps raw status + body for each, so we can verify the
// authed response shapes (campaigns/progress field names, the connect_url, the
// list of eligible channels) and tell "not enrolled" (403 on /drops/progress)
// apart from "empty 200". Wired only into cmd/kick-probe; not used by the
// daemon. Output goes to stdout.
func Probe(ctx context.Context, sess platform.Session, categorySlug string) {
	d := newHTTPDoer()

	dump := func(label, base, path string) {
		fmt.Printf("\n===== %s\nGET %s%s\n", label, base, path)
		body, status, err := d.do(ctx, sess, http.MethodGet, base+path, nil)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}
		fmt.Printf("status: %d  bytes: %d\n", status, len(body))
		// Full body for the small/decisive ones; cap the campaigns dump so a
		// huge payload stays legible while still showing the shape.
		max := 8000
		if len(body) > max {
			fmt.Printf("body[:%d]:\n%s\n…(truncated)\n", max, string(body[:max]))
		} else {
			fmt.Printf("body:\n%s\n", string(body))
		}
	}

	fmt.Fprintf(os.Stderr, "probing kick with session (account=%s, cookies=%d)\n", sess.AccountID, len(sess.Cookies))

	// 1) Authed: the drops campaigns shape (real field names, channels[], connect_url).
	dump("DROPS CAMPAIGNS (authed)", dropsBase, "/api/v1/drops/campaigns")
	// 2) Authed: progress — 403 means not enrolled/linked; 200 empty means enrolled-no-time.
	dump("DROPS PROGRESS (authed)", dropsBase, "/api/v1/drops/progress")
	// 3) Public: live channels in the campaign's category — what we COULD watch.
	if categorySlug != "" {
		dump("CATEGORY LIVESTREAMS (public)", discoveryBase, "/stream/livestreams/"+categorySlug)
	}
	// 4) Public: is the channel the watcher pinned to ("kick") actually live + what game.
	dump("CHANNEL livestream: kick (public)", discoveryBase, "/api/v2/channels/kick/livestream")
}
