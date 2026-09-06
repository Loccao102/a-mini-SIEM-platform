#!/usr/bin/env python3
"""
Sentinel Mini-SIEM & SOAR Attack Simulation Script
Simulates a multi-stage attack chain:
  Stage 1: T1110 - SSH Brute Force (Multiple failed attempts)
  Stage 2: T1078 - Valid Accounts (SSH Success)
  Stage 3: T1548.003 - Privilege Escalation (Sudo to root)

This triggers the Sentinel Correlation Engine and automatically activates
the SOAR playbook 'Auto-Contain SSH Attack Chain IP' with approval gating.
"""

import sys
import time
import json
import urllib.request
import urllib.error
import argparse

def request_json(url, data=None, headers=None, method="GET"):
    headers = headers or {}
    encoded_data = json.dumps(data).encode("utf-8") if data is not None else None
    if encoded_data:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=encoded_data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            content = resp.read().decode("utf-8")
            return resp.status, json.loads(content) if content else {}
    except urllib.error.HTTPError as e:
        content = e.read().decode("utf-8")
        try:
            return e.code, json.loads(content)
        except Exception:
            return e.code, {"error": content}
    except Exception as e:
        print(f"[!] Request error to {url}: {e}")
        return 0, {}

def main():
    parser = argparse.ArgumentParser(description="Simulate multi-stage cyber attack against Sentinel SIEM")
    parser.add_argument("--api-url", default="http://localhost:8080", help="Base URL of Sentinel SIEM API")
    parser.add_argument("--email", default="admin@example.com", help="Admin email for automatic key generation")
    parser.add_argument("--password", default="admin", help="Admin password")
    parser.add_argument("--attacker-ip", default="198.51.100.42", help="External malicious attacker IP")
    parser.add_argument("--target-host", default="srv-prod-web01", help="Target server hostname")
    parser.add_argument("--target-user", default="ubuntu", help="Target username")
    args = parser.parse_args()

    api_url = args.api_url.rstrip("/")
    print("=" * 70)
    print(" 🔥 SENTINEL SIEM & SOAR - ATTACK CHAIN SIMULATION")
    print(f" Target SIEM: {api_url}")
    print(f" Attacker IP: {args.attacker_ip} -> Victim: {args.target_user}@{args.target_host}")
    print("=" * 70)

    # 1. Authenticate to obtain JWT token
    print("\n[*] Step 1: Authenticating as SOC Admin...")
    status, auth_resp = request_json(f"{api_url}/api/v1/auth/login", {
        "email": args.email,
        "password": args.password
    }, method="POST")

    if status != 200 or "token" not in auth_resp:
        print(f"[-] Login failed (status {status}): {auth_resp}")
        print("    If running with custom credentials, pass --email and --password.")
        sys.exit(1)

    jwt_token = auth_resp["token"]
    print(f"[+] Authenticated successfully! User: {auth_resp.get('user', {}).get('email')}")

    # 2. Get or create test asset API key
    print("\n[*] Step 2: Provisioning Asset and Ingest API Key...")
    headers_auth = {"Authorization": f"Bearer {jwt_token}"}
    
    # Get assets
    status, assets = request_json(f"{api_url}/api/v1/assets", headers=headers_auth)
    asset_id = 1
    if isinstance(assets, list) and len(assets) > 0:
        asset_id = assets[0]["asset_id"]
    
    # Generate API key
    status, key_resp = request_json(f"{api_url}/api/v1/assets/{asset_id}/keys", {
        "name": f"demo-agent-{int(time.time())}",
        "expires_in_days": 30
    }, headers=headers_auth, method="POST")

    api_key = key_resp.get("raw_key")
    if not api_key:
        print(f"[-] Failed to create API key: {key_resp}")
        sys.exit(1)

    print(f"[+] Ingestion API Key generated: {api_key[:16]}... (SHA-256 protected)")

    # 3. Simulate Stage 1: SSH Brute Force (T1110)
    print("\n" + "-" * 60)
    print(" 💥 STAGE 1: Credential Access - SSH Brute Force (MITRE T1110)")
    print(f" Attacker {args.attacker_ip} attempting password guessing...")
    print("-" * 60)

    ingest_headers = {
        "X-API-Key": api_key,
        "X-Hostname": args.target_host
    }

    for attempt in range(1, 5):
        failed_log = f"sshd[{1000 + attempt}]: Failed password for {args.target_user} from {args.attacker_ip} port {50000 + attempt} ssh2"
        status, resp = request_json(f"{api_url}/api/v1/ingest", {
            "raw": failed_log,
            "source_type": "auth",
            "hostname": args.target_host
        }, headers=ingest_headers, method="POST")
        print(f"  [>] Sent failed attempt #{attempt} | HTTP {status}")
        time.sleep(0.3)

    # 4. Simulate Stage 2: Successful SSH Login (T1078)
    print("\n" + "-" * 60)
    print(" 🔓 STAGE 2: Initial Access - Valid Accounts (MITRE T1078)")
    print(f" Attacker {args.attacker_ip} successfully logged in as {args.target_user}")
    print("-" * 60)

    success_log = f"sshd[1005]: Accepted password for {args.target_user} from {args.attacker_ip} port 50005 ssh2"
    status, resp = request_json(f"{api_url}/api/v1/ingest", {
        "raw": success_log,
        "source_type": "auth",
        "hostname": args.target_host
    }, headers=ingest_headers, method="POST")
    print(f"  [>] Sent successful authentication event | HTTP {status}")
    time.sleep(0.5)

    # 5. Simulate Stage 3: Privilege Escalation (T1548.003)
    print("\n" + "-" * 60)
    print(" ⚡ STAGE 3: Privilege Escalation - Sudo Execution (MITRE T1548.003)")
    print(f" User {args.target_user} executing root shell via sudo")
    print("-" * 60)

    priv_log = f"sudo:   {args.target_user} : TTY=pts/1 ; PWD=/home/{args.target_user} ; USER=root ; COMMAND=/bin/bash"
    status, resp = request_json(f"{api_url}/api/v1/ingest", {
        "raw": priv_log,
        "source_type": "auth",
        "hostname": args.target_host
    }, headers=ingest_headers, method="POST")
    print(f"  [>] Sent privilege escalation event | HTTP {status}")

    # 6. Waiting for Correlation Engine & SOAR Execution
    print("\n" + "=" * 70)
    print(" 🛡️ SENTINEL DEFENSE - CORRELATION & SOAR RESPONSE")
    print("=" * 70)
    print("[*] Waiting 3 seconds for Redis Stream consumer, Correlation Engine and SOAR...")
    time.sleep(3)

    # Check findings
    status, findings = request_json(f"{api_url}/api/v1/detection/findings", headers=headers_auth)
    if isinstance(findings, list) and len(findings) > 0:
        latest = findings[0]
        print(f"\n[🎯 ATTACK CHAIN DETECTED!]")
        print(f"  - Title:       {latest.get('title')}")
        print(f"  - Severity:    CRITICAL (Threat Level: HIGH)")
        print(f"  - MITRE ATT&CK: T1110 -> T1078 -> T1548.003")
        print(f"  - Attacker IP: {args.attacker_ip}")
        print(f"  - Victim:      {args.target_user}@{args.target_host}")
    else:
        print("\n[*] Event ingested into stream. Polling SOAR executions...")

    # Check SOAR executions
    status, executions = request_json(f"{api_url}/api/v1/soar/executions", headers=headers_auth)
    if isinstance(executions, list) and len(executions) > 0:
        latest_exec = executions[0]
        print(f"\n[⚡ SOAR PLAYBOOK ACTIVATED!]")
        print(f"  - Execution ID: #{latest_exec.get('execution_id')}")
        print(f"  - Playbook:     Auto-Contain SSH Attack Chain IP")
        print(f"  - Target:       {latest_exec.get('target')}")
        print(f"  - Action Type:  {latest_exec.get('action_type')}")
        print(f"  - State:        {latest_exec.get('status').upper()} (Human-in-the-Loop)")
        print(f"  - Rollback TTL: 1 Hour (Automatic release)")

    print("\n" + "=" * 70)
    print(" 🚀 READY FOR DEMO / EVALUATION:")
    print(" 1. Open SOAR Dashboard:     http://localhost:3000/soar")
    print(f"    -> Click '✓ Phê duyệt Chặn' to immediately isolate IP {args.attacker_ip}")
    print(" 2. Open Incidents & Cases:  http://localhost:3000/cases")
    print("    -> Click '📄 Xuất Báo Cáo Incident' to view & print executive report")
    print("=" * 70)

if __name__ == "__main__":
    main()
