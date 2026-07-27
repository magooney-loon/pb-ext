package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	pbext "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// benchCollectionName is a throwaway collection created only to give the load
// generator something realistic to read and write in data.db. It lives in a
// throwaway data dir for the lifetime of one bench run, so public CRUD rules
// are fine here in a way they would not be for a real app.
const benchCollectionName = "bench_items"

// benchSuperuserEmail/Password authenticate the dashboard requests in the
// traffic mix, so /_/_ exercises the real dashboard render (system stats
// collectors, templates, admin-access audit) instead of the cheap login page
// an unauthenticated hit would show.
const (
	benchSuperuserEmail    = "bench@pbext.local"
	benchSuperuserPassword = "pbext-bench-load-test-0000"
)

// buildBenchServer constructs the embedded pb-ext server. dataDir is passed
// via pocketbase.Config.DefaultDataDir rather than the --dir flag, since
// PocketBase parses --dir eagerly at construction time (before Start), while
// --http is only parsed later inside Start via os.Args — see main.go.
func buildBenchServer(dataDir string) *pbext.Server {
	cfg := &pocketbase.Config{
		DefaultDataDir: dataDir,
		DefaultDev:     false,
	}

	srv := pbext.New(pbext.WithConfig(cfg), pbext.InNormalMode())

	registerBenchCollection(srv.App())
	registerBenchRoutes(srv.App())

	return srv
}

// registerBenchCollection mirrors cmd/server/collections.go's todoCollection
// pattern: a minimal collection with public rules, created once per data dir.
func registerBenchCollection(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := ensureBenchCollection(e.App); err != nil {
			app.Logger().Error("bench: failed to create bench_items collection", "error", err)
		}
		if err := ensureBenchSuperuser(e.App); err != nil {
			app.Logger().Error("bench: failed to create bench superuser", "error", err)
		}
		return e.Next()
	})
}

func ensureBenchCollection(app core.App) error {
	if existing, _ := app.FindCollectionByNameOrId(benchCollectionName); existing != nil {
		return nil
	}

	collection := core.NewBaseCollection(benchCollectionName)

	collection.Fields.Add(&core.TextField{
		Name:     "title",
		Required: true,
		Max:      200,
	})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	// A nil rule means "superuser only"; an empty-string rule means "always
	// true", i.e. public. The load generator hits these unauthenticated, so
	// they must be the latter.
	public := ""
	collection.ListRule = &public
	collection.ViewRule = &public
	collection.CreateRule = &public
	collection.UpdateRule = &public
	collection.DeleteRule = &public

	return app.Save(collection)
}

func ensureBenchSuperuser(app core.App) error {
	if existing, _ := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, benchSuperuserEmail); existing != nil {
		return nil
	}

	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return err
	}

	record := core.NewRecord(superusers)
	record.SetEmail(benchSuperuserEmail)
	record.SetPassword(benchSuperuserPassword)

	return app.Save(record)
}

// registerBenchRoutes adds a handful of plain pages so the traffic mix has
// realistic non-API "page view" hits for analytics to track, without needing
// a built frontend.
func registerBenchRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		for path, title := range benchPages {
			body := fmt.Sprintf("<html><body><h1>%s</h1></body></html>", title)
			e.Router.GET(path, func(c *core.RequestEvent) error {
				return c.HTML(http.StatusOK, body)
			})
		}
		return e.Next()
	})
}

// benchPages deliberately excludes "/": PocketBase's static file handler
// registers a "GET /{path...}" catch-all covering the whole tree, and an
// explicit "GET /" collides with it (mux pattern overlap panics at boot).
var benchPages = map[string]string{
	"/pricing": "pricing",
	"/about":   "about",
	"/docs":    "docs",
	"/faq":     "faq",
}

// waitForHealthy polls the embedded server's built-in health endpoint until
// it answers or the timeout elapses.
func waitForHealthy(addr string, timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	url := "http://" + addr + "/api/health"

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health check returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server did not become healthy within %s: %w", timeout, lastErr)
}

// authenticateBenchSuperuser exchanges the bench superuser's credentials for
// an auth token, so the dashboard requests in the traffic mix can hit the
// authenticated render path. PocketBase reads the token straight out of the
// Authorization header with no scheme prefix.
func authenticateBenchSuperuser(addr string) (string, error) {
	url := "http://" + addr + "/api/collections/" + core.CollectionNameSuperusers + "/auth-with-password"
	body := fmt.Sprintf(`{"identity":%q,"password":%q}`, benchSuperuserEmail, benchSuperuserPassword)

	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth-with-password returned %d", resp.StatusCode)
	}

	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("auth-with-password returned no token")
	}
	return parsed.Token, nil
}
