# Deploying Arena to a free, public URL (portfolio guide)

This is a complete, copy-paste walkthrough that takes Arena from source code to a
**live, public, fully-working URL** using only free tiers — assuming you have
**never used any of these tools before**. Every step says what you're doing,
what to click or paste, and what you should see.

You will deploy four pieces:

| Piece | Where | Free? |
| --- | --- | --- |
| **Frontend** (the website people visit) | **Vercel** | Yes (Hobby, persistent) |
| **API gateway** (REST + WebSocket) | **Render** (Docker web service) | Yes (spins down when idle) |
| **PostgreSQL + Redis** (data) | **Render** (Postgres + Key Value) | Yes — ⚠️ see caveats |
| **Executor** (runs submitted code in sandboxes) | **AWS EC2** (a small Linux server) | Yes for 12 months |

When you're done, the **Vercel URL is your app** — that's the link you put in
your portfolio.

```
   You (browser)
        │  https
        ▼
   Vercel  ──── Next.js frontend
        │  https REST + wss WebSocket
        ▼
   Render  ──── api-gateway ──┬─► Render PostgreSQL  (data)
                              ├─► Render Key Value   (Redis: queue, pub/sub)
                              └─► EC2 executor (gRPC :9090) ─► Docker sandboxes
```

---

## Is this plan a good idea? (read this first)

**Yes — it's a solid, genuinely-free portfolio deployment.** Three honest
caveats so nothing surprises you later:

1. **Render free Postgres is deleted after 90 days.** Fine for a time-boxed demo.
   If you want the app to keep working *indefinitely*, use **Neon** (free,
   persistent Postgres) instead — this guide shows both. Everything else
   (Vercel, the Render web service, the EC2 server) keeps running.
2. **AWS EC2 free tier lasts 12 months**, then a tiny instance costs a few
   dollars a month. One small server running 24/7 fits inside the free
   allowance (750 hours/month).
3. **The free EC2 server has only 1 GB of RAM** — not enough to compile C++/Go
   on its own. We fix this by adding **swap** (disk used as extra memory) and
   running **one submission at a time**. It's slow but it works for a demo.

> **Even better free alternative for the executor:** Oracle Cloud's *Always
> Free* ARM servers give you up to 24 GB RAM forever (no 12-month limit). It's
> more setup (ARM architecture) and you asked for EC2, so this guide uses EC2 —
> but keep Oracle in mind if you outgrow the free tier.

---

## Before you start: create the accounts (all free, ~10 minutes)

You'll need four accounts. Sign up for each now:

1. **GitHub** — you already have this (your code is at
   `github.com/CamiloRaphaelZuletaWolff/AetherJudge`).
2. **Render** — go to <https://render.com> → *Get Started* → **Sign in with
   GitHub**. Authorize Render to read your repositories.
3. **Vercel** — go to <https://vercel.com> → *Sign Up* → **Continue with
   GitHub**.
4. **AWS** — go to <https://aws.amazon.com> → *Create an AWS Account*. This one
   asks for a **credit card** (AWS requires it even for free tier; you won't be
   charged if you stay within limits). Verify your email and phone.

You'll also want a terminal on your own computer for the SSH step (Part 2):
- **Windows:** the built-in **PowerShell** (has `ssh`).
- **macOS / Linux:** the built-in **Terminal**.

> Throughout this guide, replace anything in `<ANGLE_BRACKETS>` with your real
> value. Pick one **region** and use it everywhere; this guide uses **Oregon /
> us-west-2** (US West) because Render and AWS both have it and they'll be close
> together. If you prefer another region, use it consistently.

---

## Part 1 — Create the database and Redis on Render

The gateway needs PostgreSQL (the real data) and Redis (the live queue). We'll
make both on Render so they share Render's private network.

### 1.1 PostgreSQL

1. In the Render dashboard, click **New +** (top right) → **PostgreSQL**.
2. Fill in:
   - **Name:** `aether-db`
   - **Database:** `aetherjudge`
   - **User:** `admin`
   - **Region:** **Oregon (US West)** ← remember this; everything must match it.
   - **Plan:** **Free**
3. Click **Create Database**. Wait ~1 minute until the status is **Available**.
4. Open the database → **Connect** tab. You'll see connection strings. Keep this
   tab open; you'll copy the **Internal Database URL** in Part 3.

> ⚠️ **Render's free Postgres is deleted after 90 days.** If you want permanence,
> skip this and use **Neon** instead — see [Appendix A](#appendix-a--use-neon-instead-of-render-postgres-permanent-free).

### 1.2 Redis (Render calls it "Key Value")

1. Click **New +** → **Key Value**.
2. Fill in:
   - **Name:** `aether-redis`
   - **Region:** **Oregon (US West)** — the *same* region as the database.
   - **Plan:** **Free**
3. Click **Create Key Value**. Wait until it's **Available**.
4. Open it → **Connect** tab → copy the **Internal** connection string. It looks
   like `redis://red-xxxxxxxxxxxx:6379`.
5. **Delete the `redis://` prefix** — you want only `red-xxxxxxxxxxxx:6379`. Save
   that text somewhere; it's your `REDIS_ADDR` for Part 3.

> **Do not use Upstash or other "free Redis" services here.** Arena's gateway
> talks to Redis without TLS, and those services require TLS — the gateway would
> crash on startup. Render Key Value's internal address is plain (no TLS), which
> is exactly what we need.

---

## Part 2 — Run the executor on an AWS EC2 server

The executor compiles and runs submitted code inside locked-down Docker
containers, so it needs its own Linux server with Docker. This is the longest
part — take it slowly.

### 2.1 Launch the server

1. Sign in to the [AWS Console](https://console.aws.amazon.com). In the top-right
   **region selector**, choose **US West (Oregon) us-west-2**.
2. In the search bar type **EC2** and open it. Click **Launch instance**.
3. Fill in:
   - **Name:** `aether-executor`
   - **Application and OS Images:** choose **Amazon Linux 2023** (it's
     free-tier eligible and has the `Free tier eligible` label).
   - **Instance type:** **t2.micro** (or **t3.micro**) — must say *Free tier
     eligible*.
   - **Key pair (login):** click **Create new key pair**. Name it
     `aether-key`, type **RSA**, format **.pem**. Click **Create key pair** —
     your browser downloads `aether-key.pem`. **Keep this file safe; you can't
     re-download it.**
   - **Network settings:** click **Edit**. Under *Firewall (security groups)*,
     it's creating a new security group. Make sure **Allow SSH traffic from**
     is checked and set to **My IP** (not "Anywhere"). We'll add the executor
     port in step 2.5.
   - **Configure storage:** leave the default (8–30 GB gp3 is fine and free).
4. Click **Launch instance**, then **View all instances**. Wait until
   **Instance state = Running** and **Status checks = 2/2 passed**.

### 2.2 Give it a stable public IP (Elastic IP)

By default an instance's public IP changes if it restarts. Pin it:

1. EC2 left menu → **Network & Security → Elastic IPs** → **Allocate Elastic IP
   address** → **Allocate**.
2. Select the new IP → **Actions → Associate Elastic IP address** → choose your
   `aether-executor` instance → **Associate**.
3. Copy this IP — call it **`<EXECUTOR_IP>`**. This is the address your gateway
   will talk to.

> An Elastic IP is free **while it's attached to a running instance**. If you
> stop the instance, AWS charges a few cents/day for the idle IP — so keep the
> instance running, or release the IP when you tear everything down.

### 2.3 Connect to the server (SSH)

On **your own computer**, open PowerShell (Windows) or Terminal (mac/Linux), go
to wherever `aether-key.pem` downloaded (e.g. `cd ~/Downloads`), and run:

```bash
# Lock down the key file's permissions (required by ssh)
# Windows PowerShell:
icacls aether-key.pem /inheritance:r /grant:r "$($env:USERNAME):(R)"
# macOS / Linux:
chmod 400 aether-key.pem

# Connect (replace <EXECUTOR_IP> with your Elastic IP)
ssh -i aether-key.pem ec2-user@<EXECUTOR_IP>
```

Type **yes** when asked about authenticity. You're in when the prompt changes to
something like `[ec2-user@ip-172-... ~]$`. Run the rest of Part 2 **on the
server** (inside this SSH session).

### 2.4 Set up the server (swap, Docker, the code)

Paste these blocks one at a time.

**Add 4 GB of swap** (so 1 GB of RAM can compile code without crashing):

```bash
sudo dd if=/dev/zero of=/swapfile bs=1M count=4096 status=progress
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
free -h        # you should now see ~4.0Gi in the "Swap" row
```

**Install Docker and Git, and start Docker:**

```bash
sudo dnf install -y docker git
sudo systemctl enable --now docker
sudo usermod -aG docker ec2-user
```

Now **log out and back in** so your user picks up Docker permissions:

```bash
exit
```
```bash
# from your own computer again:
ssh -i aether-key.pem ec2-user@<EXECUTOR_IP>
docker ps      # should print an empty table with no "permission denied" error
```

**Get the code:**

```bash
git clone https://github.com/CamiloRaphaelZuletaWolff/AetherJudge.git
cd AetherJudge
```

**Build the three sandbox images** (these are what submitted code runs inside).
This is the slow part on a tiny server — **expect 10–25 minutes**, and don't
worry if it looks stuck during the Go image (it's pre-compiling a cache):

```bash
docker build -t arena-sandbox-cpp:latest    -f backend/services/executor/images/cpp.Dockerfile    backend/services/executor/images
docker build -t arena-sandbox-python:latest -f backend/services/executor/images/python.Dockerfile backend/services/executor/images
docker build -t arena-sandbox-go:latest     -f backend/services/executor/images/go.Dockerfile     backend/services/executor/images
```

**Build the executor image:**

```bash
docker build -t arena-executor:latest -f backend/services/executor/Dockerfile backend/
```

**Run the executor**, telling it to talk to the server's Docker and to handle
one submission at a time (memory-safe on 1 GB):

```bash
DOCKER_GID=$(getent group docker | cut -d: -f3)
docker run -d --name executor --restart unless-stopped \
  -p 9090:9090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --group-add "$DOCKER_GID" \
  -e EXECUTOR_GRPC_ADDR=:9090 \
  -e EXECUTOR_MAX_CONCURRENT=1 \
  -e LOG_FORMAT=json \
  arena-executor:latest
```

Check it started:

```bash
docker logs executor
```

You should see a line like `grpc server listening addr=:9090`. The
`--restart unless-stopped` flag means it comes back automatically if the server
reboots. You can now close the SSH session; the executor keeps running.

### 2.5 Open the executor's port — but only to Render

The gateway (on Render) must reach the executor on port **9090**. The executor
has **no built-in authentication**, so you must **not** open this port to the
whole internet. We'll allow only Render's servers. (We come back to fill in the
exact IPs in [Part 3, step 3.4](#34-lock-the-ec2-port-to-render) once the
gateway exists.)

For now, leave port 9090 closed. Judging won't work until 3.4 — that's expected.

---

## Part 3 — Deploy the gateway on Render

The repo includes a **Blueprint** (`render.yaml`) that configures the gateway
for you.

### 3.1 Create the service from the Blueprint

1. Open this link (sign in with GitHub if asked):

   **https://dashboard.render.com/blueprint/new?repo=https://github.com/CamiloRaphaelZuletaWolff/AetherJudge**

2. Render reads `render.yaml` and shows one service, `aether-api-gateway`.
   Confirm its **Region** says **Oregon**. Click **Apply** / **Create
   resources**.
3. Render will ask you to fill in the values marked "secret". Enter:

   | Variable | What to paste |
   | --- | --- |
   | `DATABASE_URL` | the **Internal Database URL** from Part 1.1 (the `dpg-...` one) |
   | `REDIS_ADDR` | the `red-xxxx:6379` text from Part 1.2 (no `redis://`) |
   | `EXECUTOR_ADDR` | `<EXECUTOR_IP>:9090` (your Elastic IP from Part 2.2) |
   | `FRONTEND_ORIGIN` | put `https://example.com` for now — we fix it in Part 5 |

   (`JWT_SECRET` is generated automatically; `JUDGE_WORKERS` is already `2`.)
4. Click **Apply**. The first build takes a few minutes (it builds the Docker
   image). Watch the **Logs** tab.

### 3.2 Confirm it started

When the deploy is **Live**, the logs should end with
`http server listening addr=:10000`. Note your gateway URL at the top of the
page — something like `https://aether-api-gateway.onrender.com`. Call this
**`<GATEWAY_URL>`**.

Test it from your own computer:

```bash
curl <GATEWAY_URL>/healthz      # -> {"status":"ok"}
```

> If it crash-loops on `redisx: ping` or a database error, your `REDIS_ADDR` /
> `DATABASE_URL` is wrong or in a different region than the gateway. All three
> (gateway, db, key value) must be in **Oregon**. See [Troubleshooting](#troubleshooting).

### 3.3 Load the demo data

So your live app has a contest and demo users to show off:

1. On the `aether-api-gateway` service page, open the **Shell** tab.
2. Run:
   ```bash
   /seed
   ```
   This creates the "Arena Demo Contest" and users **alice** / **bob** (password
   `password123`). You should see `seeding complete` (or similar).

### 3.4 Lock the EC2 port to Render

Now that the gateway exists, find Render's outbound IPs and allow only those
into the executor:

1. On the `aether-api-gateway` page, open the **Connect** tab (or **Settings**)
   and find **Outbound IP Addresses** — a short list of IPs.
2. Go to the **AWS Console → EC2 → Instances →** select `aether-executor` **→
   Security tab →** click its **security group**.
3. **Edit inbound rules → Add rule** for **each** Render outbound IP:
   - Type: **Custom TCP**, Port range: **9090**, Source: **Custom**, then paste
     one Render IP followed by `/32` (e.g. `52.12.34.56/32`). Add one rule per
     IP. **Save rules.**
4. Leave SSH (22) restricted to **My IP**. Do **not** add a `0.0.0.0/0` rule for
   9090.

Within a minute, the gateway can reach the executor. (Until you do this,
submissions stay "queued" forever — which is the safe default.)

---

## Part 4 — Deploy the frontend on Vercel

1. Go to the [Vercel dashboard](https://vercel.com/dashboard) → **Add New… →
   Project**.
2. **Import** your `AetherJudge` repository (authorize GitHub access if asked).
3. Vercel auto-detects Next.js. Before deploying, set the project root and one
   variable:
   - **Root Directory:** click **Edit** and set it to **`frontend`** (the
     Next.js app lives there, not at the repo root). This is essential.
   - Expand **Environment Variables** and add:
     - **Name:** `NEXT_PUBLIC_API_URL`  **Value:** your `<GATEWAY_URL>` (e.g.
       `https://aether-api-gateway.onrender.com`, no trailing slash)
4. Click **Deploy**. After a couple of minutes you'll get a live URL like
   `https://aether-judge.vercel.app`. Call this **`<FRONTEND_URL>`** — **this is
   your portfolio link.**

> `NEXT_PUBLIC_API_URL` is baked in at build time. If you change it later, you
> must **redeploy** the Vercel project for it to take effect.

---

## Part 5 — Connect the frontend and gateway (CORS)

The gateway only accepts the browser if it knows the exact frontend address.

1. Back in **Render → `aether-api-gateway` → Environment**.
2. Edit **`FRONTEND_ORIGIN`** and set it to your **`<FRONTEND_URL>`** exactly —
   `https://aether-judge.vercel.app`, **no trailing slash**, and use the stable
   production URL (not a Vercel preview URL).
3. **Save changes.** Render redeploys the gateway automatically (~1–2 min).

---

## Part 6 — Verify the whole thing works

1. Open your **`<FRONTEND_URL>`** in a browser. The Arena landing page loads.

   > The first request after the gateway has been idle ~15 minutes takes
   > **30–60 seconds** (Render free services "sleep"). That's normal — it wakes
   > up and then is fast. To keep it warm, see [Keeping it awake](#keeping-the-app-awake-optional).

2. **Sign in** as `alice` / `password123` (or register a new account).
3. Open the **Arena Demo Contest** from the dashboard.
4. Pick a problem, write a solution, and click **Submit**. Within a few seconds
   the verdict badge updates and the leaderboard moves. 🎉
5. **Real-time check:** open a second browser window (incognito), sign in as
   `bob`, enter the same contest. When either user submits, **both** windows
   update live.

If submissions get a verdict, every piece is working: Vercel → Render → EC2
executor → Docker sandboxes → Redis/Postgres, all on free tiers.

---

## Keeping the app awake (optional)

Render free services sleep after 15 minutes idle, adding a cold-start delay to
the next visit. For a portfolio link people click occasionally, that's usually
fine. If you want it always-warm, create a free uptime monitor:

1. Sign up at <https://cron-job.org> (free).
2. Add a job that requests `<GATEWAY_URL>/healthz` every **10 minutes**.

(This won't make the EC2 executor sleep — it's always on.)

---

## Troubleshooting

**Gateway crash-loops with `redisx: ping (is Redis running?)`**
`REDIS_ADDR` is wrong. It must be the **bare `host:port`** from Render Key
Value's *Internal* string (no `redis://`), and the Key Value must be in the same
**region** as the gateway (Oregon).

**Gateway logs `lookup dpg-... no such host` or `lookup red-... no such host`**
The gateway is in a different **region** than the datastore. Render's internal
hostnames only resolve within one region. Recreate whichever is the odd one out
so all three are in Oregon. (Postgres also has a public *External* URL with
`?sslmode=require` that works across regions; Key Value does not.)

**Submissions stay "queued" forever**
The gateway can't reach the executor. Check: (a) the executor is running
(`docker ps` on EC2 shows `executor`), (b) the EC2 security group allows TCP
**9090** from Render's **Outbound IPs** (Part 3.4), (c) `EXECUTOR_ADDR` on Render
is exactly `<EXECUTOR_IP>:9090`.

**Submissions come back `internal_error`**
The executor reached but failed. SSH in and run `docker logs executor`. The
usual cause on the tiny server is running out of memory mid-compile — confirm
swap is on (`free -h`) and that `EXECUTOR_MAX_CONCURRENT=1`.

**Login works but you get logged out after ~15 minutes / on refresh**
The session cookie isn't surviving cross-site. Make sure the gateway's
`APP_ENV=production` (it is, via the Blueprint) and that you're on HTTPS for both
URLs. (Arena already sends the cookie as `SameSite=None; Secure` in production.)

**The browser console shows CORS errors**
`FRONTEND_ORIGIN` on Render doesn't exactly match your Vercel URL. It must be the
exact origin, `https://...vercel.app`, with no trailing slash, then redeploy.

**EC2: `permission denied` talking to Docker**
You didn't re-login after `usermod -aG docker`. Log out of SSH and back in.

---

## What this costs (and when)

| Resource | Free window | After that |
| --- | --- | --- |
| Vercel Hobby | Indefinite | Free |
| Render web service + Key Value | Indefinite | Free (sleeps when idle) |
| Render free Postgres | **90 days**, then deleted | Recreate, or move to Neon (Appendix A) |
| AWS EC2 t2.micro + Elastic IP | **12 months**, 750 hrs/month | ~\$8–10/month for one small instance |

To **tear everything down** and guarantee \$0: terminate the EC2 instance,
**release** the Elastic IP, delete the Render services, and delete the Vercel
project.

---

## Appendix A — Use Neon instead of Render Postgres (permanent free)

If you want the database to last beyond 90 days:

1. Sign up at <https://neon.tech> with GitHub (free).
2. Create a project (pick a US West region). Neon gives you a connection string
   like `postgresql://user:pass@ep-xxx.us-west-2.aws.neon.tech/neondb?sslmode=require`.
3. In Render → `aether-api-gateway` → **Environment**, set **`DATABASE_URL`** to
   that Neon string (keep the `?sslmode=require` — the gateway supports TLS to
   Postgres). Save; the gateway redeploys.
4. Re-run `/seed` from the Render **Shell** tab to load the demo data into Neon.
5. You can now skip/delete the Render `aether-db`.

Neon "scales to zero" when idle (first query after a pause takes ~1 second) and
stays free for a portfolio-sized database — no 90-day deletion.

---

## Quick reference — the values you collected

Keep these handy while following the guide:

| Placeholder | Where it came from | Example |
| --- | --- | --- |
| `<EXECUTOR_IP>` | EC2 Elastic IP (Part 2.2) | `54.12.34.56` |
| `<GATEWAY_URL>` | Render gateway URL (Part 3.2) | `https://aether-api-gateway.onrender.com` |
| `<FRONTEND_URL>` | Vercel URL (Part 4) — **your portfolio link** | `https://aether-judge.vercel.app` |
| `DATABASE_URL` | Render Postgres *Internal* URL (Part 1.1) | `postgresql://admin:...@dpg-...-a/aetherjudge` |
| `REDIS_ADDR` | Render Key Value internal `host:port` (Part 1.2) | `red-xxxx:6379` |
