# k6 latency targets report

| route    | p95 (ms) | +/- ms | p99 (ms) | target |
| -------- | -------: | -----: | -------: | :----: |
| create   |     3.78 |  -0.16 |     4.42 |   ✅   |
| edit     |     4.34 |  +0.06 |     7.53 |   ✅   |
| list     |     1.62 |  -0.18 |     1.95 |   ✅   |
| refresh  |     3.85 | -30.40 |     4.54 |   ✅   |
| sign-in  |    26.51 | -17.55 |    28.86 |   ✅   |
| sign-out |     3.50 | -15.96 |     3.73 |   ✅   |
| sign-up  |    28.28 | -16.46 |    30.69 |   ✅   |

---

**http_req_failed**: - (target ✅)
