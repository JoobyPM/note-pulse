// A parameter-friendly version of the original flow.
// • BASE_URL, START_RATE, VUS and stage targets can be overridden via env.
// • Keeps the original logic (sign-up → sign-in → refresh → sign-out).

import http from "k6/http";
import { check, sleep } from "k6";

// Base URL of the Note-Pulse API
const BASE = __ENV.BASE_URL || "http://127.0.0.1:8080/api/v1";

// -------------------------------------------------------------
// k6 execution parameters – overridable via env for flexibility
// -------------------------------------------------------------
const START_RATE = Number(__ENV.START_RATE) || 10;
const PRE_ALLOC = Number(__ENV.VUS) || 100;
const STAGE_1_TGT = Number(__ENV.RATE_STAGE1 || 50);
const STAGE_2_TGT = Number(__ENV.RATE_STAGE2 || 150);
const STAGE_3_TGT = Number(__ENV.RATE_STAGE3 || 300);

export const options = {
  tags: { script: "auth_flow" },
  scenarios: {
    auth_flow: {
      executor: "ramping-arrival-rate",
      startRate: START_RATE,
      timeUnit: "1s",
      preAllocatedVUs: PRE_ALLOC,
      stages: [
        { duration: "30s", target: STAGE_1_TGT },
        { duration: "60s", target: STAGE_2_TGT },
        { duration: "30s", target: STAGE_3_TGT },
      ],
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<400"],
  },
};

// Helper for deterministic(ish) unique e-mails
function randomEmail() {
  return `u${Math.trunc(Math.random() * 1e12)}@example.com`;
}

export default function () {
  const email = randomEmail();
  const password = "Passw0rd123";

  // 1. sign-up
  let res = http.post(
    `${BASE}/auth/sign-up`,
    JSON.stringify({ email, password }),
    {
      headers: { "Content-Type": "application/json" },
    },
  );
  check(res, { "sign-up ok": (r) => r.status >= 200 && r.status < 300 });

  // 2. sign-in (rate-limited path – but the limiter is muted by env override)
  res = http.post(`${BASE}/auth/sign-in`, JSON.stringify({ email, password }), {
    headers: { "Content-Type": "application/json" },
  });
  check(res, { "sign-in ok": (r) => r.status >= 200 && r.status < 300 });

  if (res.status === 429) {
    console.log("Rate limit exceeded");
    return;
  }

  if (res.status >= 200 && res.status < 300) {
    const token = res.json("token");
    const refresh = res.json("refresh_token");
    // 3. refresh
    res = http.post(
      `${BASE}/auth/refresh`,
      JSON.stringify({ refresh_token: refresh }),
      {
        headers: { "Content-Type": "application/json" },
      },
    );
    check(res, { "refresh ok": (r) => r.status >= 200 && r.status < 300 });

    // 4. sign-out
    res = http.post(
      `${BASE}/auth/sign-out`,
      JSON.stringify({ refresh_token: refresh }),
      {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      },
    );
    check(res, { "sign-out ok": (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(1);
}
