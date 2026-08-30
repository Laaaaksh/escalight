// Records a genuine end-to-end walkthrough of Escalight: builds the real
// binary, boots it against a throwaway SQLite file and a minimal local SMTP
// relay, drives the actual web UI (plus one real call to the ingest API) with
// Playwright, and converts the captured video into docs/assets/demo.mp4 and
// docs/assets/demo.gif. See README.md in this directory for usage.
"use strict";

const { chromium } = require("playwright");
const { spawn, spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const http = require("http");
const { startFakeSmtp } = require("./fake-smtp");

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const TMP_DIR = path.join(__dirname, ".tmp");
const BINARY = path.join(TMP_DIR, "escalight-demo-bin");
const DB_PATH = path.join(TMP_DIR, "demo.db");
const VIDEO_DIR = path.join(TMP_DIR, "video");
const ASSETS_DIR = path.join(REPO_ROOT, "docs", "assets");
const RAW_VIDEO = path.join(TMP_DIR, "demo-raw.webm");
const OUT_MP4 = path.join(ASSETS_DIR, "demo.mp4");
const OUT_GIF = path.join(ASSETS_DIR, "demo.gif");

const BASE_URL = "http://localhost:8080";
const SMTP_HOST = "127.0.0.1";
const SMTP_PORT = 1025;

function pause(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function nowUtcDatetimeLocal() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
}

function waitForHttp(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  const attempt = () =>
    new Promise((resolve, reject) => {
      const req = http.get(url, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", reject);
      req.setTimeout(1000, () => req.destroy(new Error("request timeout")));
    });

  return (async () => {
    while (Date.now() < deadline) {
      try {
        await attempt();
        return;
      } catch {
        await pause(300);
      }
    }
    throw new Error(`server never became reachable at ${url}`);
  })();
}

async function submitAndWait(page, clickTarget) {
  await Promise.all([page.waitForNavigation(), page.click(clickTarget)]);
}

async function driveUI(serverReadyAt) {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
    recordVideo: { dir: VIDEO_DIR, size: { width: 1280, height: 800 } },
  });
  const page = await context.newPage();

  // 1. First-run setup creates the admin account and logs in.
  await page.goto(BASE_URL);
  await page.waitForURL("**/setup");
  await page.fill("#name", "Alex Rivera");
  await page.fill("#email", "alex@escalight.dev");
  await page.fill("#password", "DemoPass123!");
  await pause(1000);
  await submitAndWait(page, 'button:has-text("Create admin account")');
  await page.waitForURL("**/incidents");
  await pause(800);

  // 2. A second responder for the secondary escalation step.
  await page.goto(BASE_URL + "/users");
  await page.fill("#name", "Jordan Lee");
  await page.fill("#email", "jordan@escalight.dev");
  await page.fill("#password", "DemoPass123!");
  await pause(900);
  await submitAndWait(page, 'button:has-text("Add user")');
  await pause(1000);

  // 3. An on-call schedule with a two-person daily rotation.
  await page.goto(BASE_URL + "/schedules/new");
  await page.fill("#name", "Primary On-call");
  await pause(700);
  await submitAndWait(page, 'button:has-text("Create")');
  await pause(800);

  await page.fill("#start_at", nowUtcDatetimeLocal());
  await page.click('button:has-text("+ Add person")');
  await page.click('button:has-text("+ Add person")');
  const orderSelects = page.locator('#user-order select[name="user_order"]');
  await orderSelects.nth(0).selectOption({ label: "Alex Rivera" });
  await orderSelects.nth(1).selectOption({ label: "Jordan Lee" });
  await pause(900);
  await submitAndWait(page, 'button:has-text("Save rotation")');
  await pause(2000);

  // 4. A two-level escalation policy: page the on-call schedule first, then
  // the secondary responder if nobody acknowledges.
  await page.goto(BASE_URL + "/policies/new");
  await page.fill("#name", "Production API Escalation");
  await page.fill(
    "#description",
    "Page the primary on-call immediately; escalate to the secondary responder if nobody acknowledges."
  );
  await page.fill('input[name="step_0_wait"]', "0");
  await page
    .locator('select[name="step_0_target"]')
    .selectOption({ label: "On-call: Primary On-call" });
  await pause(700);
  await page.click('button:has-text("+ Add step")');
  await page.fill('input[name="step_1_wait"]', "0");
  await page
    .locator('select[name="step_1_target"]')
    .selectOption({ label: "Jordan Lee" });
  await pause(2000);
  await submitAndWait(page, 'button:has-text("Save policy")');
  await pause(1200);

  // 5. A service tying an ingest webhook to that policy.
  await page.goto(BASE_URL + "/services/new");
  await page.fill("#name", "Production API");
  await page
    .locator("#policy_id")
    .selectOption({ label: "Production API Escalation" });
  await pause(700);
  await submitAndWait(page, 'button:has-text("Create")');
  await pause(1000);

  const genericUrl = (
    await page.locator("code.copy-key").first().textContent()
  ).trim();

  // The escalation engine ticks on a fixed 15s cadence from server boot
  // (internal/engine.Engine.Interval), not from when an incident is created.
  // Firing at a random phase in that cycle can land right before a tick,
  // making the level-1 and level-2 pages appear within a couple of seconds
  // of each other — a real but misleading-looking "notified at once". Wait
  // until just after a tick boundary so the incident starts a fresh ~15s
  // window and the handoff reads as a genuine, visible timeout.
  const TICK_MS = 15000;
  const sinceReady = Date.now() - serverReadyAt;
  const alignWait = TICK_MS - (sinceReady % TICK_MS) + 1500;
  await pause(alignWait);

  // 6. Trigger a real incident through the ingest API, exactly as a monitoring
  // tool would.
  const triggerResponse = await context.request.post(genericUrl, {
    data: {
      title: "Payment API returning 5xx on checkout",
      description: "/checkout is failing ~40% of requests over the last 2 minutes.",
    },
  });
  const triggerBody = await triggerResponse.json();
  const incidentId = triggerBody.incident_id;
  if (!incidentId) {
    throw new Error(`ingest API did not return an incident_id: ${JSON.stringify(triggerBody)}`);
  }

  // 7. Level 1: it arrives and pages only the primary on-call. Hold here so
  // the viewer clearly reads "triggered, one person notified" before anything
  // else happens.
  await page.goto(BASE_URL + "/incidents");
  await pause(1800);
  await page.goto(BASE_URL + "/incidents/" + incidentId);
  await pause(3500);

  // 8. Level 1 goes unacknowledged for the real ~15s window above, then the
  // engine's next tick escalates to the secondary responder. Reload once that
  // has genuinely happened and hold again so the new "notified Jordan" line
  // reads as a distinct second event, not a simultaneous one.
  await pause(13500);
  await page.reload();
  await pause(3200);

  // 9. Acknowledge promptly — well within the next 15s tick — so the policy
  // never runs out of steps, then resolve and let the final timeline sit.
  await submitAndWait(page, 'button:has-text("Acknowledge")');
  await pause(1800);
  await submitAndWait(page, 'button:has-text("Resolve")');
  await pause(4000);

  await context.close();
  await browser.close();

  const files = fs.readdirSync(VIDEO_DIR).filter((f) => f.endsWith(".webm"));
  if (files.length === 0) {
    throw new Error("Playwright did not produce a video file");
  }
  fs.renameSync(path.join(VIDEO_DIR, files[0]), RAW_VIDEO);
}

function run(cmd, args, opts) {
  const result = spawnSync(cmd, args, { stdio: "inherit", ...opts });
  if (result.status !== 0) {
    throw new Error(`${cmd} ${args.join(" ")} failed with status ${result.status}`);
  }
}

function convertAssets() {
  fs.mkdirSync(ASSETS_DIR, { recursive: true });

  console.log("Encoding docs/assets/demo.mp4 ...");
  run("ffmpeg", [
    "-y",
    "-i", RAW_VIDEO,
    "-vf", "scale=1280:-2",
    "-c:v", "libx264",
    "-pix_fmt", "yuv420p",
    "-movflags", "+faststart",
    OUT_MP4,
  ]);

  console.log("Encoding docs/assets/demo.gif ...");
  const palette = path.join(TMP_DIR, "palette.png");
  const fps = 12;
  const scale = "scale=960:-2:flags=lanczos";
  run("ffmpeg", [
    "-y",
    "-i", RAW_VIDEO,
    "-vf", `fps=${fps},${scale},palettegen`,
    palette,
  ]);
  run("ffmpeg", [
    "-y",
    "-i", RAW_VIDEO,
    "-i", palette,
    "-lavfi", `fps=${fps},${scale}[x];[x][1:v]paletteuse`,
    OUT_GIF,
  ]);

  const gifSize = fs.statSync(OUT_GIF).size;
  console.log(`demo.gif size: ${(gifSize / (1024 * 1024)).toFixed(2)} MB`);
  if (gifSize > 10 * 1024 * 1024) {
    throw new Error(
      `demo.gif is ${(gifSize / (1024 * 1024)).toFixed(2)} MB, over the 10 MB budget — shorten the walkthrough or drop fps`
    );
  }
}

async function main() {
  fs.rmSync(TMP_DIR, { recursive: true, force: true });
  fs.mkdirSync(TMP_DIR, { recursive: true });
  fs.mkdirSync(VIDEO_DIR, { recursive: true });

  console.log("Building escalight...");
  run("go", ["build", "-o", BINARY, "."], { cwd: REPO_ROOT });

  console.log("Starting fake SMTP relay on 127.0.0.1:1025...");
  const { server: smtpServer } = await startFakeSmtp(SMTP_HOST, SMTP_PORT);

  console.log("Starting escalight on :8080...");
  const child = spawn(BINARY, ["serve"], {
    cwd: TMP_DIR,
    env: {
      ...process.env,
      ESCALIGHT_ADDR: ":8080",
      ESCALIGHT_BASE_URL: BASE_URL,
      ESCALIGHT_DB_PATH: DB_PATH,
      ESCALIGHT_SMTP_HOST: SMTP_HOST,
      ESCALIGHT_SMTP_PORT: String(SMTP_PORT),
      ESCALIGHT_SMTP_FROM: "escalight@localhost",
    },
    stdio: ["ignore", "inherit", "inherit"],
  });

  let cleanedUp = false;
  const cleanup = () => {
    if (cleanedUp) return;
    cleanedUp = true;
    try {
      child.kill("SIGTERM");
    } catch {}
    try {
      smtpServer.close();
    } catch {}
  };
  process.on("exit", cleanup);
  process.on("SIGINT", () => {
    cleanup();
    process.exit(1);
  });

  try {
    await waitForHttp(BASE_URL + "/", 15000);
    const serverReadyAt = Date.now();
    await driveUI(serverReadyAt);
  } finally {
    cleanup();
  }

  convertAssets();
  console.log("Done. Verify with: ffprobe docs/assets/demo.gif");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
