// loadtest/scripts/user_flow.js
//
// Isolated-route benchmark - one endpoint at a time.
//
// Timeline (defaults):
//   ┌─ idle 60 s ─┐
//   ├─ PATCH   100 rps ⋅15 s ┤
//   ├─ gap 15 s ┤
//   ├─ GET     100 rps ⋅15 s ┤
//   ├─ gap 15 s ┤
//   ├─ POST    100 rps ⋅15 s ┤   (/notes)
//   ├─ gap 15 s ┤
//   ├─ POST     10 rps ⋅15 s ┤   (/auth/refresh)  <── refreshed logic
//   ├─ gap 15 s ┤
//   ├─ POST      5 rps ⋅15 s ┤   (/auth/sign-up)
//   ├─ gap 15 s ┤
//   ├─ POST      5 rps ⋅15 s ┤   (/auth/sign-in)
//   ├─ gap 15 s ┤
//   └─ POST      5 rps ⋅15 s ┘   (/auth/sign-out)

import http from "k6/http";
import { check, fail } from "k6";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";


const BASE = __ENV.BASE_URL || "http://127.0.0.1:8080/api/v1";

const WARM_UP_SEC = Number(__ENV.WARM_UP_SEC) || 60;
const STEP_SEC = Number(__ENV.STEP_SEC) || 15;
const GAP_SEC = Number(__ENV.GAP_SEC) || 15;

const EDIT_RATE = Number(__ENV.EDIT_RATE) || 100;
const LIST_RATE = Number(__ENV.LIST_RATE) || 100;
const CREATE_RATE = Number(__ENV.CREATE_RATE) || 100;
const REFRESH_RATE = Number(__ENV.REFRESH_RATE) || 10;
const SIGNUP_RATE = Number(__ENV.SIGNUP_RATE) || 5;
const SIGNIN_RATE = Number(__ENV.SIGNIN_RATE) || 5;
const SIGNOUT_RATE = Number(__ENV.SIGNOUT_RATE) || 5;

const secs = (n) => `${n}s`;
const hdrJSON = { "Content-Type": "application/json" };

/* ── setup – seed fixtures once ───────────────────────────────────────── */

export function setup() {
  // 1) seed account for /notes (shared)
  const seedEmail = "seed_notes@example.com";
  const seedPass = "Passw0rd123";

  http.post(
    `${BASE}/auth/sign-up`,
    JSON.stringify({ email: seedEmail, password: seedPass }),
    { headers: hdrJSON },
  );

  const signin = http.post(
    `${BASE}/auth/sign-in`,
    JSON.stringify({ email: seedEmail, password: seedPass }),
    { headers: hdrJSON },
  );

  check(signin, { "seed sign-in ok": (r) => r.status === 200 }) ||
    fail("seed sign-in failed - cannot continue");

  const seedToken = signin.json("token");

  // 2) one note to PATCH repeatedly
  const noteRes = http.post(
    `${BASE}/notes`,
    JSON.stringify({ title: "seed", body: "first" }),
    { headers: { ...hdrJSON, Authorization: `Bearer ${seedToken}` } },
  );

  check(noteRes, { "seed note created": (r) => r.status === 201 }) ||
    fail("failed to create seed note - cannot continue");

  const noteId = noteRes.json("note.id");

  // 3) reusable auth account for isolated sign-in / sign-out tests
  const authEmail = "auth_user@example.com";
  const authPass = "Passw0rd123";

  http.post(
    `${BASE}/auth/sign-up`,
    JSON.stringify({ email: authEmail, password: authPass }),
    { headers: hdrJSON },
  );

  return {
    seedEmail, // new → used by refreshExec per-VU
    seedPass, // new → used by refreshExec per-VU
    seedToken,
    noteId,
    authEmail,
    authPass,
  };
}

/* ── scenario factory ─────────────────────────────────────────────────── */

function isolated(rate, order, exec) {
  const offset = WARM_UP_SEC + order * (STEP_SEC + GAP_SEC);
  return {
    executor: "constant-arrival-rate",
    startTime: secs(offset),
    rate,
    timeUnit: "1s",
    duration: secs(STEP_SEC),
    preAllocatedVUs: Math.max(10, rate * 2),
    exec,
  };
}

/* ── k6 options ───────────────────────────────────────────────────────── */

export const options = {
  summaryTrendStats: ["avg", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    editRoute: isolated(EDIT_RATE, 0, "editExec"),
    listRoute: isolated(LIST_RATE, 1, "listExec"),
    createRoute: isolated(CREATE_RATE, 2, "createExec"),
    refreshRoute: isolated(REFRESH_RATE, 3, "refreshExec"), // updated
    signupRoute: isolated(SIGNUP_RATE, 4, "signUpExec"),
    signinRoute: isolated(SIGNIN_RATE, 5, "signInExec"),
    signoutRoute: isolated(SIGNOUT_RATE, 6, "signOutExec"),
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],

    "http_req_duration{route:edit}": ["p(95)<400", "p(99)<600"],
    "http_req_duration{route:list}": ["p(95)<400", "p(99)<600"],
    "http_req_duration{route:create}": ["p(95)<400", "p(99)<600"],
    "http_req_duration{route:refresh}": ["p(95)<400", "p(99)<600"],
    "http_req_duration{route:sign-up}": ["p(95)<800", "p(99)<1200"],
    "http_req_duration{route:sign-in}": ["p(95)<800", "p(99)<1200"],
    "http_req_duration{route:sign-out}": ["p(95)<800", "p(99)<1200"],
  },
  tags: { script: "user_flow" },
};

http.setResponseCallback(
  http.expectedStatuses({ min: 200, max: 299 }),
);

/* ── exec functions ───────────────────────────────────────────────────── */

export function editExec(data) {
  const r = http.patch(
    `${BASE}/notes/${data.noteId}`,
    JSON.stringify({ body: "upd" }),
    {
      headers: { ...hdrJSON, Authorization: `Bearer ${data.seedToken}` },
      tags: { route: "edit" },
    },
  );
  check(r, { "edit ok": (res) => res.status === 200 });
}

export function listExec(data) {
  const r = http.get(`${BASE}/notes?limit=50`, {
    headers: { Authorization: `Bearer ${data.seedToken}` },
    tags: { route: "list" },
  });
  check(r, { "list ok": (res) => res.status >= 200 && res.status < 300 });
}

export function createExec(data) {
  const r = http.post(
    `${BASE}/notes`,
    JSON.stringify({ title: uuidv4(), body: "body" }),
    {
      headers: { ...hdrJSON, Authorization: `Bearer ${data.seedToken}` },
      tags: { route: "create" },
    },
  );
  check(r, { "create ok": (res) => res.status >= 200 && res.status < 300 });
}

/* ── NEW: per-VU fresh refresh-token ──────────────────────────────────── */
export function refreshExec(data) {
  /* Each iteration:
     1) sign-in as the shared seed user → unique refresh-token for this VU
     2) immediately exchange it on /auth/refresh
     This preserves one-time-use semantics and keeps VUs isolated. */

  const signin = http.post(
    `${BASE}/auth/sign-in`,
    JSON.stringify({ email: data.seedEmail, password: data.seedPass }),
    { headers: hdrJSON },
  );

  check(signin, { "seed re-sign ok": (r) => r.status === 200 }) ||
    fail("seed re-sign-in failed - refresh path cannot proceed");

  const refreshTok = signin.json("refresh_token");

  const r = http.post(
    `${BASE}/auth/refresh`,
    JSON.stringify({ refresh_token: refreshTok }),
    { headers: hdrJSON, tags: { route: "refresh" } },
  );

  check(r, { "refresh ok": (res) => res.status >= 200 && res.status < 300 });
}

export function signUpExec() {
  const email = `u_${uuidv4()}@ex.com`;
  const r = http.post(
    `${BASE}/auth/sign-up`,
    JSON.stringify({ email, password: "Passw0rd123" }),
    { headers: hdrJSON, tags: { route: "sign-up" } },
  );
  check(r, { "sign-up ok": (res) => res.status >= 200 && res.status < 300 });
}

export function signInExec(data) {
  const r = http.post(
    `${BASE}/auth/sign-in`,
    JSON.stringify({ email: data.authEmail, password: data.authPass }),
    { headers: hdrJSON, tags: { route: "sign-in" } },
  );
  check(r, { "sign-in ok": (res) => res.status >= 200 && res.status < 300 });
}

export function signOutExec(data) {
  // fresh tokens for clean sign-out
  const auth = http.post(
    `${BASE}/auth/sign-in`,
    JSON.stringify({ email: data.authEmail, password: data.authPass }),
    { headers: hdrJSON },
  );
  const tok = auth.json("token");
  const ref = auth.json("refresh_token");

  const r = http.post(
    `${BASE}/auth/sign-out`,
    JSON.stringify({ refresh_token: ref }),
    {
      headers: { ...hdrJSON, Authorization: `Bearer ${tok}` },
      tags: { route: "sign-out" },
    },
  );
  check(r, { "sign-out ok": (res) => res.status >= 200 && res.status < 300 });
}
