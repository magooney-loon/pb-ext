# pb-ext bench — 2 vCPU / 4GB (Hetzner CX22-class, current cheapest)

**Host:** fedora (constrained to 2 vCPU / 4GB via `systemd-run --user --scope`)
**Date:** 2026-07-27T19:34:06Z
**Command:**

```sh
systemd-run --user --scope -p CPUQuota=200% -p MemoryMax=4G -- \
  env GOMAXPROCS=2 pbext-bench -workers=5,20,50 -duration=15s
```

## Summary

Same shape as the 1 vCPU run, roughly 1.5x the headroom: throughput peaks at 5 concurrent connections (~2,458 req/s) and falls off past that as CPU pins at ~190–192%. RAM peaked at 171MB of a 4GB budget — again nowhere near the limit. **CPU is the ceiling here too**, and error rate stayed under 0.3% even at full saturation.

## Throughput & Latency

| Stage | Workers | Duration | Requests | Req/s | P50 | P90 | P99 | Max | Err% |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 5 | 15s | 36,875 | 2458.2 | 1.05ms | 8.39ms | 16.78ms | 67.11ms | 0.01% |
| 2 | 20 | 15s | 19,410 | 1293.3 | 16.78ms | 33.55ms | 67.11ms | 268.44ms | 0.04% |
| 3 | 50 | 15s | 15,218 | 1014.2 | 33.55ms | 134.22ms | 268.44ms | 1.07s | 0.30% |

*Latency figures are bucketed (power-of-two upper-bound) estimates, not exact quantiles — see `cmd/bench/histogram.go`.*

## Resource Usage

| Stage | CPU% (avg / peak) | RSS (avg / peak) | Disk% (avg / peak) |
|---|---|---|---|
| 1 | 147.9% / 182.7% | 57.6MiB / 71.9MiB | 1.1% / 1.1% |
| 2 | 188.2% / 190.1% | 101.0MiB / 119.5MiB | 1.2% / 1.2% |
| 3 | 191.4% / 192.2% | 142.2MiB / 170.9MiB | 1.2% / 1.3% |

## Database Growth (whole run)

| File | Before | After | Delta |
|---|---|---|---|
| `data.db` | 4.0KiB | 1.7MiB | +1.7MiB |
| `data.db-wal` | 289.7KiB | 0B | -289.7KiB |
| `auxiliary.db` | 4.0KiB | 28.7MiB | +28.7MiB |
| `auxiliary.db-wal` | 76.5KiB | 0B | -76.5KiB |

- `bench_items` rows: 14,307 (as of end of load, before the final flush)
- `_analytics` rows: 3,320 (tracked views: 16,790; as of end of load, before the final flush)

WAL files checkpoint cleanly to 0B on graceful shutdown in both cases.

## Per-Endpoint Breakdown (final stage, 50 workers)

| Endpoint | Requests | Errors |
|---|---|---|
| `GET /api/health` | 3,786 | 0 |
| `GET page` | 3,093 | 0 |
| `GET records` | 3,040 | 5 |
| `POST records` | 3,072 | 41 |
| `GET dashboard` | 1,485 | 0 |
| `GET 404` (expected) | 742 | 0 |

<details>
<summary>Full live progress log</summary>

```
[stage 1/3 workers=5   ]    2s/15s      8723 req    4361.1 rps  p99=8.388608ms err= 0.00%  cpu= 17.5%  rss=33.8MiB   data.db=+320.0KiB  aux.db= +2.6MiB
[stage 1/3 workers=5   ]    4s/15s     14845 req    3710.9 rps  p99=8.388608ms err= 0.00%  cpu=142.3%  rss=49.4MiB   data.db=+464.0KiB  aux.db= +5.5MiB
[stage 1/3 workers=5   ]    6s/15s     19932 req    3321.9 rps  p99=8.388608ms err= 0.00%  cpu=163.5%  rss=53.3MiB   data.db=+560.0KiB  aux.db= +7.9MiB
[stage 1/3 workers=5   ]    8s/15s     24230 req    3028.4 rps  p99=16.777216ms err= 0.00%  cpu=172.3%  rss=58.3MiB   data.db=+660.0KiB  aux.db= +9.4MiB
[stage 1/3 workers=5   ]   10s/15s     28298 req    2829.7 rps  p99=16.777216ms err= 0.00%  cpu=177.1%  rss=66.1MiB   data.db=+756.0KiB  aux.db=+10.8MiB
[stage 1/3 workers=5   ]   12s/15s     31917 req    2658.9 rps  p99=16.777216ms err= 0.00%  cpu=180.3%  rss=71.9MiB   data.db=+856.0KiB  aux.db=+12.3MiB
[stage 1/3 workers=5   ]   14s/15s     35297 req    2521.0 rps  p99=16.777216ms err= 0.00%  cpu=182.7%  rss=70.4MiB   data.db=+904.0KiB  aux.db=+13.7MiB
[stage 2/3 workers=20  ]    2s/15s      2902 req    1450.0 rps  p99=67.108864ms err= 0.00%  cpu=185.7%  rss=91.4MiB   data.db=+56.0KiB  aux.db= +1.3MiB
[stage 2/3 workers=20  ]    4s/15s      5776 req    1442.7 rps  p99=67.108864ms err= 0.00%  cpu=186.8%  rss=92.2MiB   data.db=+116.0KiB  aux.db= +2.5MiB
[stage 2/3 workers=20  ]    6s/15s      8404 req    1400.5 rps  p99=67.108864ms err= 0.00%  cpu=187.7%  rss=93.6MiB   data.db=+184.0KiB  aux.db= +3.3MiB
[stage 2/3 workers=20  ]    8s/15s     11236 req    1403.8 rps  p99=67.108864ms err= 0.00%  cpu=188.4%  rss=95.4MiB   data.db=+236.0KiB  aux.db= +4.5MiB
[stage 2/3 workers=20  ]   10s/15s     13635 req    1363.1 rps  p99=67.108864ms err= 0.00%  cpu=189.0%  rss=108.8MiB  data.db=+292.0KiB  aux.db= +5.6MiB
[stage 2/3 workers=20  ]   12s/15s     16046 req    1336.9 rps  p99=67.108864ms err= 0.00%  cpu=189.6%  rss=106.5MiB  data.db=+344.0KiB  aux.db= +6.5MiB
[stage 2/3 workers=20  ]   14s/15s     18244 req    1303.1 rps  p99=67.108864ms err= 0.00%  cpu=190.1%  rss=119.5MiB  data.db=+400.0KiB  aux.db= +7.4MiB
[stage 3/3 workers=50  ]    2s/15s      2168 req    1082.0 rps  p99=268.435456ms err= 0.00%  cpu=190.4%  rss=121.8MiB  data.db=+52.0KiB  aux.db= +1.0MiB
[stage 3/3 workers=50  ]    4s/15s      4387 req    1095.4 rps  p99=268.435456ms err= 0.00%  cpu=190.8%  rss=124.8MiB  data.db=+120.0KiB  aux.db= +1.8MiB
[stage 3/3 workers=50  ]    6s/15s      6499 req    1083.1 rps  p99=268.435456ms err= 0.00%  cpu=191.1%  rss=134.9MiB  data.db=+156.0KiB  aux.db= +2.7MiB
[stage 3/3 workers=50  ]    8s/15s      8504 req    1062.6 rps  p99=268.435456ms err= 0.00%  cpu=191.5%  rss=158.8MiB  data.db=+216.0KiB  aux.db= +3.6MiB
[stage 3/3 workers=50  ]   10s/15s     10514 req    1051.4 rps  p99=268.435456ms err= 0.00%  cpu=191.8%  rss=132.2MiB  data.db=+256.0KiB  aux.db= +4.4MiB
[stage 3/3 workers=50  ]   12s/15s     12432 req    1035.4 rps  p99=268.435456ms err= 0.00%  cpu=192.0%  rss=170.9MiB  data.db=+296.0KiB  aux.db= +5.0MiB
[stage 3/3 workers=50  ]   14s/15s     14305 req    1021.6 rps  p99=268.435456ms err= 0.00%  cpu=192.2%  rss=152.1MiB  data.db=+332.0KiB  aux.db= +5.7MiB
```

</details>
