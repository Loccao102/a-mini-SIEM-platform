#!/usr/bin/env bash
set -e

echo "================================================================="
echo " 🔥 Sentinel SIEM & SOAR - Quick Attack Simulation"
echo "================================================================="

SIEM_URL="${SIEM_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"
ATTACKER_IP="198.51.100.42"
TARGET_HOST="srv-prod-web01"
TARGET_USER="ubuntu"

echo "[*] Step 1: Logging in as Admin..."
TOKEN=$(curl -s -X POST "$SIEM_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "[-] Login failed. Ensure SIEM backend is running at $SIEM_URL"
  exit 1
fi
echo "[+] Authenticated successfully."

echo "[*] Step 2: Generating Ingest API Key..."
API_KEY=$(curl -s -X POST "$SIEM_URL/api/v1/assets/1/keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"bash-demo-agent","expires_in_days":7}' | grep -o '"raw_key":"[^"]*' | cut -d'"' -f4)

if [ -z "$API_KEY" ]; then
  echo "[-] Failed to generate key."
  exit 1
fi
echo "[+] Ingest Key generated."

echo "[*] Step 3: Sending 4 Failed SSH Logins (MITRE T1110)..."
for i in 1 2 3 4; do
  curl -s -X POST "$SIEM_URL/api/v1/ingest" \
    -H "X-API-Key: $API_KEY" \
    -H "X-Hostname: $TARGET_HOST" \
    -H "Content-Type: application/json" \
    -d "{\"raw\":\"sshd[100$i]: Failed password for $TARGET_USER from $ATTACKER_IP port 5000$i ssh2\",\"source_type\":\"auth\",\"hostname\":\"$TARGET_HOST\"}" > /dev/null
  echo "  [>] Failed login #$i sent."
  sleep 0.2
done

echo "[*] Step 4: Sending 1 Successful SSH Login (MITRE T1078)..."
curl -s -X POST "$SIEM_URL/api/v1/ingest" \
  -H "X-API-Key: $API_KEY" \
  -H "X-Hostname: $TARGET_HOST" \
  -H "Content-Type: application/json" \
  -d "{\"raw\":\"sshd[1005]: Accepted password for $TARGET_USER from $ATTACKER_IP port 50005 ssh2\",\"source_type\":\"auth\",\"hostname\":\"$TARGET_HOST\"}" > /dev/null
echo "  [>] Successful login sent."
sleep 0.3

echo "[*] Step 5: Sending Privilege Escalation Event (MITRE T1548.003)..."
curl -s -X POST "$SIEM_URL/api/v1/ingest" \
  -H "X-API-Key: $API_KEY" \
  -H "X-Hostname: $TARGET_HOST" \
  -H "Content-Type: application/json" \
  -d "{\"raw\":\"sudo:   $TARGET_USER : TTY=pts/1 ; PWD=/home/$TARGET_USER ; USER=root ; COMMAND=/bin/bash\",\"source_type\":\"auth\",\"hostname\":\"$TARGET_HOST\"}" > /dev/null
echo "  [>] Privilege escalation sent."

echo ""
echo "================================================================="
echo " 🛡️ Attack Chain Ingested! Correlation Engine & SOAR Triggered."
echo " 👉 Open Web Dashboard: http://localhost:3000/soar"
echo "    to approve the containment action and isolate IP $ATTACKER_IP."
echo "================================================================="
