#!/usr/bin/env python3
"""Black-box probe of the upstream's free-tier meter (test harness, VPS only).

Each case is one request from the gateway's own egress addresses or from a
tunnel exit, through whatever client surface it asks for. The goal is to find
which input actually decides the answer: the address, the credentials, the
identity headers, the body shape or the TLS fingerprint.
"""
import argparse
import json
import random
import string
import subprocess
import sys
import time

UPSTREAM = "https://opencode.ai/inference/v1"
CHAT = UPSTREAM + "/chat/completions"
MODELS = UPSTREAM + "/models"
UA = "opencode/latest/2.0.22/cli"
TOOLS_PATH = "/root/aurora-gateway/configs/extensions/opencode/tools/opencode-tools.json"
LOCAL_IPS = ["193.23.210.27", "193.23.197.41", "31.77.160.138", "64.188.68.192", "64.188.70.246", "77.239.120.61"]
SIDECAR_PORT = 8097


def sid():
    head = format((int(time.time() * 1000) << 12) & ((1 << 64) - 1), "x")[:12]
    alpha = string.ascii_letters + string.digits
    return "ses_" + head + "".join(random.choice(alpha) for _ in range(14))


def identity():
    s = sid()
    return {
        "x-opencode-session": s,
        "x-opencode-session-id": s,
        "x-session-affinity": s,
        "x-session-id": s,
        "x-opencode-client": "cli",
        "x-opencode-project": "".join(random.choice("0123456789abcdef") for _ in range(40)),
        "b3": "".join(random.choice("0123456789abcdef") for _ in range(32)) + "-"
              + "".join(random.choice("0123456789abcdef") for _ in range(16)),
    }


def chat_body(model="glm-5.3-flash-free", tools=False, cli=False, stream=False):
    body = {"model": model, "messages": [{"role": "user", "content": "hi"}], "max_tokens": 8, "stream": stream}
    if tools:
        body["tools"] = json.load(open(TOOLS_PATH))
        body["tool_choice"] = "auto"
    if cli:
        body["cli"] = True
    return body


def classify(status, text):
    text = text or ""
    if "FreeUsageLimitError" in text:
        return "429-free"
    if "FreeTierError" in text:
        return "403-signature"
    if "authentication_error" in text or "Invalid credential" in text:
        return "401-cred"
    if status == 404:
        return "404-node"
    if status == 200:
        return "200-OK"
    return f"{status}"


def direct(case):
    """One request through the shipped uTLS probe binary."""
    req = {"url": case.get("url", CHAT), "method": case.get("method", "POST"),
           "preset": case.get("preset", "chrome120"),
           "headers": case.get("headers", {}), "bind_ip": case.get("bind_ip", ""),
           "proxy": case.get("proxy", "")}
    if req["method"] == "POST":
        req["body"] = json.dumps(case.get("body") or chat_body())
    res = subprocess.run(["/root/probe"], input=json.dumps(req) + "\n",
                         capture_output=True, text=True)
    try:
        ans = json.loads(res.stdout.strip().splitlines()[-1])
    except Exception:
        return "probe-error", (res.stdout or res.stderr)[:80]
    if ans.get("error"):
        return "transport", str(ans["error"])[:80]
    return classify(ans.get("status"), ans.get("body")), f'{ans.get("status")} {ans.get("elapsed_ms")}ms'


class Sidecar:
    """The real sidecar, isolated on its own port with no proxy rotation."""

    def __init__(self):
        env = {"AURORA_SIDECAR_PORT": str(SIDECAR_PORT), "AURORA_SIDECAR_BIND_PROXIES": "",
               "AURORA_SIDECAR_BIND_PROXIES_FILE": "/tmp/none.json",
               "AURORA_SIDECAR_OVERRIDES_PATH": "/tmp/none-overrides.json",
               "AURORA_SIDECAR_UPSTREAM_URL": UPSTREAM,
               "AURORA_SIDECAR_USER_AGENT": UA, "AURORA_SIDECAR_DEFAULT_AUTH": "Bearer public"}
        args = ["docker", "exec", "-d"]
        for k, v in env.items():
            args += ["-e", f"{k}={v}"]
        args += ["aurora-gateway", "/usr/local/bin/bun", "/opt/sidecar/adapter.js"]
        subprocess.run(args, capture_output=True)
        for _ in range(30):
            if subprocess.run(["curl", "-sS", "-m", "2", "-o", "/dev/null",
                               f"http://127.0.0.1:{SIDECAR_PORT}/health"],
                              capture_output=True).returncode == 0:
                if subprocess.run(["docker", "exec", "aurora-gateway", "sh", "-c",
                                   "ss -ltn 2>/dev/null | grep -q 8097 || true"], capture_output=True).returncode == 0:
                    time.sleep(1)
                return
            time.sleep(0.5)
        raise SystemExit("sidecar did not start")

    def ask(self, case):
        headers = dict(case.get("headers") or {})
        fh = [x for k, v in headers.items() for x in ("-H", f"{k}: {v}")]
        body = json.dumps(case.get("body") or chat_body())
        r = subprocess.run(
            ["curl", "-sS", "-m", "70", "-o", "/tmp/sc.json", "-w", "%{http_code} %{time_total}s",
             "-X", "POST", f"http://127.0.0.1:{SIDECAR_PORT}/v1/chat/completions",
             "-H", "Content-Type: application/json"] + fh + ["-d", body],
            capture_output=True, text=True)
        try:
            text = open("/tmp/sc.json", encoding="utf-8", errors="replace").read()
        except Exception:
            text = ""
        code = r.stdout.split()[0] if r.stdout else ""
        return classify(int(code) if code.isdigit() else 0, text), r.stdout.strip()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("suite", choices=["ips", "models", "tokens", "headless", "surface", "session", "fingerprint"])
    args = ap.parse_args()

    if args.suite == "ips":
        cases = [(f"ip {ip}", {"bind_ip": ip, "headers": {"Authorization": "Bearer public", "User-Agent": UA, **identity()}}, "direct")
                 for ip in LOCAL_IPS]
    elif args.suite == "models":
        cases = [(f"models {ip}", {"url": MODELS, "method": "GET", "bind_ip": ip,
                                   "headers": {"Authorization": "Bearer public", "User-Agent": UA}}, "direct")
                 for ip in LOCAL_IPS[:3]]
    elif args.suite == "tokens":
        cases = [("no auth", {"bind_ip": LOCAL_IPS[0], "headers": {"User-Agent": UA}}, "direct"),
                 ("empty bearer", {"bind_ip": LOCAL_IPS[0], "headers": {"Authorization": "Bearer ", "User-Agent": UA}}, "direct"),
                 ("public", {"bind_ip": LOCAL_IPS[0], "headers": {"Authorization": "Bearer public", "User-Agent": UA}}, "direct"),
                 ("lowercase scheme", {"bind_ip": LOCAL_IPS[0], "headers": {"Authorization": "bearer public", "User-Agent": UA}}, "direct")]
    elif args.suite == "headless":
        cases = [("identity + tools", {**identity(), "Authorization": "Bearer public", "User-Agent": UA}, chat_body(tools=True, cli=True)),
                 ("identity only", {**identity(), "Authorization": "Bearer public"}, chat_body()),
                 ("bare", {"Authorization": "Bearer public"}, chat_body())]
        side = Sidecar()
        for label, headers, body in cases:
            kind, detail = side.ask({"headers": headers, "body": body})
            print(f"  {label:<26} | {kind:<14} | {detail}", flush=True)
        return
    elif args.suite == "session":
        ids = identity()
        cases = [("same id twice #1", ids), ("same id twice #2", ids), ("fresh id", identity())]
        side = Sidecar()
        for label, h in cases:
            kind, detail = side.ask({"headers": {**h, "Authorization": "Bearer public"}, "body": chat_body()})
            print(f"  {label:<26} | {kind:<14} | {detail}", flush=True)
        return
    elif args.suite == "fingerprint":
        cases = [(f"preset {p}", {"preset": p, "bind_ip": LOCAL_IPS[0],
                                  "headers": {"Authorization": "Bearer public", "User-Agent": UA, **identity()}}, "direct")
                 for p in ("chrome120", "firefox120", "safari", "ios", "randomized")]
        cases.append(("golang-default-tls", {"preset": "golang", "bind_ip": LOCAL_IPS[0],
                                             "headers": {"Authorization": "Bearer public", "User-Agent": UA, **identity()}}, "direct"))
    else:
        raise SystemExit("unknown suite")

    for label, case, _ in cases:
        kind, detail = direct(case)
        print(f"  {label:<26} | {kind:<14} | {detail}", flush=True)


if __name__ == "__main__":
    main()
