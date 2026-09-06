param (
    [string]$SiemUrl = "http://localhost:8080",
    [string]$Email = "admin@example.com",
    [string]$Password = "admin",
    [string]$AttackerIp = "198.51.100.42",
    [string]$TargetHost = "srv-prod-web01",
    [string]$TargetUser = "ubuntu"
)

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host " Sentinel SIEM / SOAR - Multi-Stage Attack Simulation (Windows)" -ForegroundColor Red
Write-Host "=================================================================" -ForegroundColor Cyan

# Step 1: Login as Admin
Write-Host "`n[*] Step 1: Logging in as Admin ($Email)..." -ForegroundColor Yellow
$loginBody = @{
    email = $Email
    password = $Password
} | ConvertTo-Json

try {
    $loginResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
    $token = $loginResp.token
    Write-Host "[+] Authenticated successfully! Token acquired." -ForegroundColor Green
} catch {
    $errMsg = $_.Exception.Message
    Write-Host "[-] Login failed: $errMsg" -ForegroundColor Red
    Write-Host "    Make sure backend is running on $SiemUrl and admin user exists."
    exit 1
}

$authHeaders = @{
    "Authorization" = "Bearer $token"
}

# Step 2: Enroll fleet agent for target host & get Asset ID
Write-Host "`n[*] Step 2: Enrolling Fleet Agent for $TargetHost..." -ForegroundColor Yellow
$agentId = "ps-demo-agent-01"
$enrollBody = @{
    agent_id = $agentId
    hostname = $TargetHost
    os_type = "linux"
    criticality = "high"
    policy_name = "linux-baseline"
    policy_version = 1
    source_types = @("linux_sshd", "syslog")
} | ConvertTo-Json

try {
    $enrollResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/fleet/agents" -Method Post -Headers $authHeaders -Body $enrollBody -ContentType "application/json"
    $assetId = $enrollResp.asset_id
    Write-Host "[+] Agent enrolled: $agentId (Asset ID: $assetId)" -ForegroundColor Green
} catch {
    # If already enrolled, find asset ID
    $assets = Invoke-RestMethod -Uri "$SiemUrl/api/v1/assets" -Method Get -Headers $authHeaders
    $found = $assets | Where-Object { $_.hostname -eq $TargetHost }
    if ($found) {
        $assetId = $found.asset_id
        Write-Host "[+] Host already registered (Asset ID: $assetId)" -ForegroundColor Green
    } else {
        $errMsg = $_.Exception.Message
        Write-Host "[-] Failed to enroll asset: $errMsg" -ForegroundColor Red
        exit 1
    }
}

# Step 3: Generate Ingest API Key
Write-Host "`n[*] Step 3: Generating Ingest API Key for Asset $assetId..." -ForegroundColor Yellow
try {
    $keyResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/assets/$assetId/keys" -Method Post -Headers $authHeaders -Body "{}" -ContentType "application/json"
    $apiKey = $keyResp.api_key
    Write-Host "[+] Ingest Key generated: $($apiKey.Substring(0, [Math]::Min(16, $apiKey.Length)))..." -ForegroundColor Green
} catch {
    $errMsg = $_.Exception.Message
    Write-Host "[-] Failed to generate key: $errMsg" -ForegroundColor Red
    exit 1
}

$ingestHeaders = @{
    "Authorization" = "Bearer $apiKey"
}

# Step 4: Send 4 Failed SSH Logins (MITRE T1110)
Write-Host "`n[*] Step 4: Sending 4 Failed SSH Logins (MITRE T1110)..." -ForegroundColor Yellow
for ($i = 1; $i -le 4; $i++) {
    $port = 50000 + $i
    $pidNum = 1000 + $i
    $logMsg = "sshd[$pidNum]: Failed password for $TargetUser from $AttackerIp port $port ssh2"
    $payload = @{
        message = $logMsg
        source_type = "linux_sshd"
        hostname = $TargetHost
        agent_id = $agentId
    } | ConvertTo-Json
    
    $res = Invoke-RestMethod -Uri "$SiemUrl/api/v1/ingest" -Method Post -Headers $ingestHeaders -Body $payload -ContentType "application/json"
    Write-Host "  [>] Failed login #$i sent (Stream: $($res.stream_id))" -ForegroundColor Gray
    Start-Sleep -Milliseconds 250
}

# Step 5: Send 1 Successful SSH Login (MITRE T1078)
Write-Host "`n[*] Step 5: Sending 1 Successful SSH Login (MITRE T1078)..." -ForegroundColor Yellow
$successMsg = "sshd[1005]: Accepted password for $TargetUser from $AttackerIp port 50005 ssh2"
$payload = @{
    message = $successMsg
    source_type = "linux_sshd"
    hostname = $TargetHost
    agent_id = $agentId
} | ConvertTo-Json
$res = Invoke-RestMethod -Uri "$SiemUrl/api/v1/ingest" -Method Post -Headers $ingestHeaders -Body $payload -ContentType "application/json"
Write-Host "  [>] Successful login sent (Stream: $($res.stream_id))" -ForegroundColor Gray
Start-Sleep -Milliseconds 300

# Step 6: Send Privilege Escalation Event (MITRE T1548.003)
Write-Host "`n[*] Step 6: Sending Privilege Escalation (MITRE T1548.003)..." -ForegroundColor Yellow
$privMsg = "sudo:   $TargetUser : TTY=pts/1 ; PWD=/home/$TargetUser ; USER=root ; COMMAND=/bin/bash"
$payload = @{
    message = $privMsg
    source_type = "linux_sshd"
    hostname = $TargetHost
    agent_id = $agentId
} | ConvertTo-Json
$res = Invoke-RestMethod -Uri "$SiemUrl/api/v1/ingest" -Method Post -Headers $ingestHeaders -Body $payload -ContentType "application/json"
Write-Host "  [>] Privilege escalation sent (Stream: $($res.stream_id))" -ForegroundColor Gray

Write-Host "`n=================================================================" -ForegroundColor Cyan
Write-Host " Attack Chain Ingested! Correlation Engine and SOAR Triggered." -ForegroundColor Green
Write-Host " Open Web Dashboard: http://localhost:3000/soar" -ForegroundColor Yellow
Write-Host " to approve the containment action and isolate IP $AttackerIp." -ForegroundColor Yellow
Write-Host "=================================================================" -ForegroundColor Cyan
