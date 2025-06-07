# k6 latency targets report

| route    | p95 (ms) | +/- ms | p99 (ms) | target |
| -------- | -------: | -----: | -------: | :----: |
| create   |     3.94 |  -0.71 |     6.26 |   ✅   |
| edit     |     4.28 |  -1.14 |     7.98 |   ✅   |
| list     |     1.80 |  -0.73 |     2.31 |   ✅   |
| refresh  |    34.25 |  +2.53 |    36.24 |   ✅   |
| sign-in  |    44.06 |  -7.44 |    45.40 |   ✅   |
| sign-out |    19.46 |  +0.04 |    23.04 |   ✅   |
| sign-up  |    44.74 |  -7.22 |    48.13 |   ✅   |

---

**http_req_failed**: - (target ✅)
