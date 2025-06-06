# k6 SLA report

| route    | p95 (ms) | p99 (ms) | SLA |
| -------- | -------: | -------: | :-: |
| create   |     4.65 |     6.85 | ✅  |
| edit     |     5.42 |     9.06 | ✅  |
| list     |     2.53 |     4.66 | ✅  |
| refresh  |    31.72 |    33.75 | ✅  |
| sign-in  |    51.50 |    53.77 | ✅  |
| sign-out |    19.42 |    20.14 | ✅  |
| sign-up  |    51.96 |    54.80 | ✅  |

---

**http_req_failed**: - (SLA ✅)
