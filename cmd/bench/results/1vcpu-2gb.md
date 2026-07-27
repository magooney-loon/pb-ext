# pb-ext bench — 1 vCPU / 2GB ("$5 VPS" class, e.g. Hetzner CX11)

**Host:** fedora (constrained to 1 vCPU / 2GB via `systemd-run --user --scope`)
**Date:** 2026-07-27T19:33:12Z
**Command:**

```sh
systemd-run --user --scope -p CPUQuota=100% -p MemoryMax=2G -- \
  env GOMAXPROCS=1 pbext-bench -workers=5,20,50 -duration=15s
```

## Summary

Throughput peaks at just 5 concurrent connections (~1,636 req/s) and then *declines* as concurrency rises further — CPU pins at 94–96% and latency balloons instead. RAM never came close to mattering (84MB peak of a 2GB budget). **CPU is the ceiling on this tier, not memory or the database.** Even fully saturated, error rate stayed under 0.5% — it degrades by slowing down, not by falling over.

## Throughput & Latency

| Stage | Workers | Duration | Requests | Req/s | P50 | P90 | P99 | Max | Err% |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 5 | 15s | 24,537 | 1635.8 | 4.19ms | 8.39ms | 16.78ms | 134.22ms | 0.02% |
| 2 | 20 | 15s | 13,593 | 904.7 | 33.55ms | 67.11ms | 67.11ms | 134.22ms | 0.15% |
| 3 | 50 | 15s | 10,559 | 702.8 | 134.22ms | 134.22ms | 268.44ms | 268.44ms | 0.44% |

*Latency figures are bucketed (power-of-two upper-bound) estimates, not exact quantiles — see `cmd/bench/histogram.go`.*

## Resource Usage

| Stage | CPU% (avg / peak) | RSS (avg / peak) | Disk% (avg / peak) |
|---|---|---|---|
| 1 | 74.9% / 91.9% | 47.8MiB / 58.6MiB | 1.1% / 1.1% |
| 2 | 94.3% / 95.2% | 67.8MiB / 71.7MiB | 1.1% / 1.1% |
| 3 | 95.8% / 96.1% | 80.2MiB / 84.3MiB | 1.1% / 1.2% |

## Database Growth (whole run)

| File | Before | After | Delta |
|---|---|---|---|
| `data.db` | 4.0KiB | 1.2MiB | +1.2MiB |
| `data.db-wal` | 289.7KiB | 0B | -289.7KiB |
| `auxiliary.db` | 4.0KiB | 19.5MiB | +19.5MiB |
| `auxiliary.db-wal` | 76.5KiB | 0B | -76.5KiB |

- `bench_items` rows: 9,724 (as of end of load, before the final flush)
- `_analytics` rows: 2,387 (tracked views: 12,169; as of end of load, before the final flush)

WAL files checkpoint cleanly to 0B on graceful shutdown in both cases.

## Per-Endpoint Breakdown (final stage, 50 workers)

| Endpoint | Requests | Errors |
|---|---|---|
| `GET /api/health` | 2,608 | 13 |
| `GET page` | 2,058 | 10 |
| `GET records` | 2,174 | 11 |
| `POST records` | 2,096 | 6 |
| `GET dashboard` | 1,114 | 5 |
| `GET 404` (expected) | 509 | 1 |

<details>
<summary>Full live progress log</summary>

```
[stage 1/3 workers=5   ]    2s/15s      5374 req    2682.3 rps  p99=8.388608ms err= 0.00%  cpu= 13.0%  rss=33.0MiB   data.db=+272.0KiB  aux.db= +1.5MiB
[stage 1/3 workers=5   ]    4s/15s      9577 req    2392.5 rps  p99=8.388608ms err= 0.00%  cpu= 71.1%  rss=40.5MiB   data.db=+364.0KiB  aux.db= +3.4MiB
[stage 1/3 workers=5   ]    6s/15s     12975 req    2162.1 rps  p99=8.388608ms err= 0.00%  cpu= 81.9%  rss=43.7MiB   data.db=+416.0KiB  aux.db= +4.9MiB
[stage 1/3 workers=5   ]    8s/15s     15989 req    1998.3 rps  p99=16.777216ms err= 0.00%  cpu= 86.6%  rss=49.6MiB   data.db=+508.0KiB  aux.db= +6.1MiB
[stage 1/3 workers=5   ]   10s/15s     18637 req    1863.6 rps  p99=16.777216ms err= 0.00%  cpu= 89.2%  rss=53.9MiB   data.db=+556.0KiB  aux.db= +7.3MiB
[stage 1/3 workers=5   ]   12s/15s     21130 req    1760.7 rps  p99=16.777216ms err= 0.00%  cpu= 90.8%  rss=55.4MiB   data.db=+604.0KiB  aux.db= +7.9MiB
[stage 1/3 workers=5   ]   14s/15s     23494 req    1677.9 rps  p99=16.777216ms err= 0.00%  cpu= 91.9%  rss=58.6MiB   data.db=+652.0KiB  aux.db= +9.0MiB
[stage 2/3 workers=20  ]    2s/15s      2033 req    1016.3 rps  p99=67.108864ms err= 0.00%  cpu= 93.2%  rss=63.9MiB   data.db=+48.0KiB  aux.db= +1.0MiB
[stage 2/3 workers=20  ]    4s/15s      4002 req    1000.3 rps  p99=67.108864ms err= 0.00%  cpu= 93.7%  rss=63.1MiB   data.db=+48.0KiB  aux.db= +1.5MiB
[stage 2/3 workers=20  ]    6s/15s      5852 req     974.9 rps  p99=67.108864ms err= 0.00%  cpu= 94.1%  rss=67.1MiB   data.db=+100.0KiB  aux.db= +2.4MiB
[stage 2/3 workers=20  ]    8s/15s      7634 req     954.1 rps  p99=67.108864ms err= 0.00%  cpu= 94.5%  rss=69.7MiB   data.db=+140.0KiB  aux.db= +3.3MiB
[stage 2/3 workers=20  ]   10s/15s      9373 req     937.3 rps  p99=67.108864ms err= 0.00%  cpu= 94.7%  rss=68.1MiB   data.db=+188.0KiB  aux.db= +3.8MiB
[stage 2/3 workers=20  ]   12s/15s     11047 req     920.4 rps  p99=67.108864ms err= 0.00%  cpu= 95.0%  rss=70.7MiB   data.db=+236.0KiB  aux.db= +4.6MiB
[stage 2/3 workers=20  ]   14s/15s     12711 req     907.8 rps  p99=67.108864ms err= 0.00%  cpu= 95.2%  rss=71.7MiB   data.db=+236.0KiB  aux.db= +5.4MiB
[stage 3/3 workers=50  ]    2s/15s      1390 req     694.6 rps  p99=268.435456ms err= 0.00%  cpu= 95.4%  rss=75.1MiB   data.db=     +0B  aux.db=+952.0KiB
[stage 3/3 workers=50  ]    4s/15s      2831 req     707.3 rps  p99=134.217728ms err= 0.00%  cpu= 95.6%  rss=77.1MiB   data.db=+52.0KiB  aux.db= +1.3MiB
[stage 3/3 workers=50  ]    6s/15s      4289 req     713.8 rps  p99=134.217728ms err= 0.00%  cpu= 95.7%  rss=79.0MiB   data.db=+100.0KiB  aux.db= +2.1MiB
[stage 3/3 workers=50  ]    8s/15s      5732 req     716.5 rps  p99=134.217728ms err= 0.00%  cpu= 95.8%  rss=82.1MiB   data.db=+100.0KiB  aux.db= +2.5MiB
[stage 3/3 workers=50  ]   10s/15s      7067 req     706.6 rps  p99=268.435456ms err= 0.00%  cpu= 95.9%  rss=80.0MiB   data.db=+152.0KiB  aux.db= +2.9MiB
[stage 3/3 workers=50  ]   12s/15s      8396 req     699.6 rps  p99=268.435456ms err= 0.00%  cpu= 96.0%  rss=84.1MiB   data.db=+152.0KiB  aux.db= +3.7MiB
[stage 3/3 workers=50  ]   14s/15s      9887 req     706.0 rps  p99=268.435456ms err= 0.00%  cpu= 96.1%  rss=84.3MiB   data.db=+192.0KiB  aux.db= +4.1MiB
```

</details>
