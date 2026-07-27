# System Metrics

Every collector in `core/monitoring/` wraps gopsutil, whose field names rarely mean what they look like. The rules below are each pinned by a regression test — read them before touching a collector or the metric templates.

**Memory** (`memory.go`): `Free` and `Available` are not the same thing and the dashboard must show `Available`.

On Linux `Free` is `MemFree` — completely untouched pages — which sits near zero on any warm machine because the kernel fills RAM with page cache. `Available` is `MemAvailable`: what a new allocation can actually obtain, cache included. Showing `Free` understates usable memory by an order of magnitude (0.7 GB vs 20.7 GB on a 31 GB box).

`Used + Free != Total`. gopsutil computes `Used = Total - Free - Buffers - Cached` (with `Cached` including `SReclaimable`), so the missing remainder is `Cached` — which is why `MemoryInfo` tracks it. `UsedPercent` is `Used/Total` and is correct as-is.

`core/monitoring/memory_test.go` verifies `Total` and `Available` against `/proc/meminfo` directly (Linux-gated) and pins the `Used + Free + Cached ≈ Total` identity.

**CPU** (`cpu.go`): call `cpu.Percent` with `percpu=true`. With `percpu=false` gopsutil returns a *single* aggregate value; assigning it positionally leaves every entry after the first at zero, and since the dashboard averages across entries the meter then reads low by a factor of the CPU count. `assignUsage` also falls back to the mean when `cpu.Info` and `cpu.Percent` disagree on entry count, so the average stays correct either way. Temperature is optional — a host without sensors is not a collection failure.

**Disk** (`disk.go`): `Used + Free != Total`. `Free` excludes reserved blocks (5% on ext4 by default), so usage is `Used/(Used+Free)` — what `df` prints — not `Used/Total`. gopsutil computes this correctly as `UsedPercent`; `SystemStats.DiskUsagePercent` carries it through so templates never recompute it.

`CollectDiskInfo` takes the path to measure, and the dashboard passes `app.DataDir()` — the filesystem holding `pb_data` is the one that decides when the database runs out of room. `/` is often a small read-only image layer in a container, or an ostree overlay on an atomic host, and reports something unrelated (100% of 0.0 GB on one such machine, versus 11.3% of 1.9 TB for the data directory). An empty or unqueryable path falls back to `DefaultDiskPath`, and `DiskInfo.Path` / `SystemStats.DiskPath` report whichever was actually measured — the Disk card shows it so the figure is never ambiguous. `CollectSystemStats` takes the path too, and its refresh cache is keyed on it so a changed path never returns another filesystem's figures.

**Network** (`network.go`): identify loopback by the interface *flag*, never by looking for `"lo"` in the name — that substring also matches `wlo1` and `eno1` (systemd predictable names for onboard wireless/ethernet), which silently drops a machine's primary interface and its byte counters. Byte totals are cumulative since boot, not throughput.

**Temperature** (`temperature.go`): classify ambient *before* system, because `IsSystemTemp` also accepts `"ambient"` and would otherwise make `AmbientTemp` permanently zero. Each category keeps the highest reading rather than whichever sensor came last — sensor order is not guaranteed and groups like coretemp report a package sensor plus one per core. `SystemStats.Temperatures` holds the classified readings so template helpers never re-read sensors mid-render.

**Requests** (`requests.go`): `RequestStats` keeps monotonic counters, a scalar rate and a fixed 100-entry ring — and **no per-path breakdown**. It used to hold a `map[path]*PathStats`: keyed by attacker-chosen input, one entry per 404 from the static file handler, paths bounded only by the ~8KB header limit, never evicted, and — decisively — never read by anything. A scanner walking long junk URLs bought permanent server memory for free. If per-path stats are ever wanted, they need the treatment `core/analytics` gives the same hazard (`MaxDistinctPaths` plus the `/*` overflow bucket, a path-length cap) *and* a consumer to justify them. `TestRequestStats_TrackRequestIsBounded` pins this.

`Totals()` counts 4xx and 5xx separately, because alerting on the sum fires on any bot sweeping for `/wp-admin`. Prefer it over `GetRequestRate()` for anything that computes a rate — see the note in the Alerts section.

**Process** (`process.go`): `OpenFiles` alone is not an alertable figure. The soft `RLIMIT_NOFILE` is 1024 on some hosts and 1048576 on others, so the same count means "about to fail" or "idle" depending on the machine — `OpenFilesPercent` is the ratio against `OpenFilesLimit`, and that is what the saturation rule watches. `OpenFilesLimit` is 0 where the ceiling is unknown (Windows, or a failed `Getrlimit`), which every consumer must treat as "skip the check", never as "0%". The lookup is build-tagged: `rlimit_unix.go` / `rlimit_other.go`.

**Runtime** (`runtime.go`): `HeapObjects` is a *count* of live objects, not a size. Dividing it by 1048576 and labelling it MB renders a meaningless near-zero figure; use `AllocatedBytes` for heap size and the `formatCount` template func for counts. `LastGCDuration` uses the documented `PauseNs[(NumGC+255)%256]` ring-buffer index.
