# k6 latency targets report

| route    | p95 (ms) | +/- ms | p99 (ms) | target |
| -------- | -------: | -----: | -------: | :----: |
| create   |     4.94 |  +1.16 |     6.92 |   ✅   |
| edit     |    11.21 |  +6.87 |    41.84 |   ✅   |
| list     |     5.14 |  +3.52 |    44.34 |   ✅   |
| refresh  |     3.57 |  -0.28 |     4.11 |   ✅   |
| sign-in  |    31.92 |  +5.41 |    42.61 |   ✅   |
| sign-out |     4.53 |  +1.03 |    10.70 |   ✅   |
| sign-up  |    35.45 |  +7.17 |    36.11 |   ✅   |

---

**http_req_failed**: - (target ✅)
