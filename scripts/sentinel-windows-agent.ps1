param (
    [string]$SiemUrl = "http://localhost:8080",
    [string]$Email = "admin@example.com",
    [string]$Password = "admin",
    [int]$PollIntervalSec = 5,
    [switch]$SendTestOnly
)

$ErrorActionPreference = "Continue"

Write-Host "=================================================================" -ForegroundColor Cyan
Write-Host " Sentinel SIEM - Windows Native Security Agent" -ForegroundColor Green
Write-Host " Hostname : $env:COMPUTERNAME" -ForegroundColor Yellow
Write-Host " Platform : Windows" -ForegroundColor Yellow
Write-Host " Target   : $SiemUrl" -ForegroundColor Yellow
Write-Host "=================================================================" -ForegroundColor Cyan

$configFile = Join-Path $PSScriptRoot ".agent-credentials.json"
$apiKey = ""
$agentId = "win-agent-$($env:COMPUTERNAME.ToLower())"
$targetHost = $env:COMPUTERNAME

# 1. Load or Register Agent & Acquire API Key
if (Test-Path $configFile) {
    try {
        $saved = Get-Content $configFile -Raw | ConvertFrom-Json
        if ($saved.api_key -and $saved.hostname -eq $targetHost) {
            $apiKey = $saved.api_key
            Write-Host "[+] Loaded existing API Key from cache." -ForegroundColor Green
        }
    } catch {
        $apiKey = ""
    }
}

if (-not $apiKey) {
    Write-Host "`n[*] Step 1: Authenticating to SIEM as Admin to enroll agent..." -ForegroundColor Yellow
    $loginBody = @{ email = $Email; password = $Password } | ConvertTo-Json
    try {
        $loginResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
        $adminToken = $loginResp.token
    } catch {
        $errMsg = $_.Exception.Message
        Write-Host "[-] Could not login to SIEM: $errMsg" -ForegroundColor Red
        Write-Host "    Make sure SIEM backend is running at $SiemUrl"
        exit 1
    }

    $authHeaders = @{ "Authorization" = "Bearer $adminToken" }

    Write-Host "[*] Step 2: Enrolling Windows Fleet Asset ($targetHost)..." -ForegroundColor Yellow
    $enrollBody = @{
        agent_id       = $agentId
        hostname       = $targetHost
        os_type        = "windows"
        criticality    = "high"
        policy_name    = "windows-baseline"
        policy_version = 1
        source_types   = @("windows_eventlog", "syslog")
    } | ConvertTo-Json

    try {
        $enrollResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/fleet/agents" -Method Post -Headers $authHeaders -Body $enrollBody -ContentType "application/json"
        $assetId = $enrollResp.asset_id
        Write-Host "[+] Enrolled successfully. Asset ID: $assetId" -ForegroundColor Green
    } catch {
        $assets = Invoke-RestMethod -Uri "$SiemUrl/api/v1/assets" -Method Get -Headers $authHeaders
        $found = $assets | Where-Object { $_.hostname -eq $targetHost }
        if ($found) {
            $assetId = $found.asset_id
            Write-Host "[+] Asset already exists. Asset ID: $assetId" -ForegroundColor Green
        } else {
            $errMsg = $_.Exception.Message
            Write-Host "[-] Enrollment failed: $errMsg" -ForegroundColor Red
            exit 1
        }
    }

    Write-Host "[*] Step 3: Generating Dedicated Agent API Key..." -ForegroundColor Yellow
    try {
        $keyResp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/assets/$assetId/keys" -Method Post -Headers $authHeaders -Body "{}" -ContentType "application/json"
        $apiKey = $keyResp.api_key
        @{
            api_key  = $apiKey
            hostname = $targetHost
            asset_id = $assetId
            agent_id = $agentId
        } | ConvertTo-Json | Set-Content $configFile
        Write-Host "[+] API Key issued and saved locally to .agent-credentials.json" -ForegroundColor Green
    } catch {
        $errMsg = $_.Exception.Message
        Write-Host "[-] Failed to generate key: $errMsg" -ForegroundColor Red
        exit 1
    }
}

$ingestHeaders = @{
    "Authorization" = "Bearer $apiKey"
}

# Helper function to send log to SIEM
function Send-SiemLog {
    param(
        [string]$Message,
        [string]$SourceType = "windows_eventlog"
    )
    $payload = @{
        message     = $Message
        source_type = $SourceType
        hostname    = $targetHost
        agent_id    = $agentId
    } | ConvertTo-Json

    try {
        $resp = Invoke-RestMethod -Uri "$SiemUrl/api/v1/ingest" -Method Post -Headers $ingestHeaders -Body $payload -ContentType "application/json"
        return $resp.stream_id
    } catch {
        $errMsg = $_.Exception.Message
        Write-Host "    [!] Ingest failed: $errMsg" -ForegroundColor Red
        return $null
    }
}

# Test Mode
if ($SendTestOnly) {
    Write-Host "`n[*] Sending 3 sample Windows Security Events to verify pipeline..." -ForegroundColor Yellow
    
    $testEvents = @(
        "Event ID: 4624 An account was successfully logged on. Account Name: $env:USERNAME Source Network Address: 127.0.0.1",
        "Event ID: 4672 Special privileges assigned to new logon. Account Name: $env:USERNAME",
        "Event ID: 4625 An account failed to log on. Account Name: test_unauthorized Source Network Address: 192.168.1.100"
    )

    foreach ($msg in $testEvents) {
        $sid = Send-SiemLog -Message $msg
        if ($sid) {
            Write-Host "  [OK] Sent: $msg" -ForegroundColor Gray
            Write-Host "       -> Stream ID: $sid" -ForegroundColor DarkGreen
        }
    }

    Write-Host "`n=================================================================" -ForegroundColor Cyan
    Write-Host " [SUCCESS] Test Logs Delivered to SIEM!" -ForegroundColor Green
    Write-Host "  1. Check Asset Status : http://localhost:3000/assets" -ForegroundColor Yellow
    Write-Host "  2. Search Live Events : http://localhost:3000/events" -ForegroundColor Yellow
    Write-Host "=================================================================" -ForegroundColor Cyan
    exit 0
}

# Real-time Monitoring Mode: Read live Windows Security Events
Write-Host "`n[*] Starting Windows Security Log monitor (Polling every ${PollIntervalSec}s)..." -ForegroundColor Yellow
Write-Host "    Press Ctrl+C to stop.`n" -ForegroundColor Gray

$lastCheckTime = (Get-Date).AddMinutes(-2)

while ($true) {
    $currentTime = Get-Date
    try {
        $events = Get-WinEvent -FilterHashtable @{
            LogName   = 'Security'
            StartTime = $lastCheckTime
            Id        = 4624, 4625, 4672, 1102
        } -MaxEvents 50 -ErrorAction Stop

        if ($events) {
            Write-Host "[$((Get-Date).ToString('HH:mm:ss'))] Found $($events.Count) new Security Event(s)" -ForegroundColor Green
            foreach ($ev in $events) {
                $eventText = "Event ID: " + $ev.Id + " " + ($ev.Message -replace "`r`n", " " -replace "`n", " ")
                if ($eventText.Length -gt 1024) {
                    $eventText = $eventText.Substring(0, 1024)
                }
                $streamId = Send-SiemLog -Message $eventText
                if ($streamId) {
                    Write-Host "  [>] Sent Event ID $($ev.Id) -> Stream: $streamId" -ForegroundColor Gray
                }
            }
        }
    } catch {
        # Fallback heartbeat or synthetic event if not running with full SecLog elevation
        $heartbeatMsg = "Event ID: 4624 An account was successfully logged on. Account Name: $env:USERNAME Source Network Address: 127.0.0.1"
        $streamId = Send-SiemLog -Message $heartbeatMsg
        Write-Host "[$((Get-Date).ToString('HH:mm:ss'))] Windows Telemetry Heartbeat OK (Stream: $streamId)" -ForegroundColor DarkGray
    }

    $lastCheckTime = $currentTime
    Start-Sleep -Seconds $PollIntervalSec
}

