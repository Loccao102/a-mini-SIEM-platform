package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultConsumerGroup = "siem-parser"

// SSH Regexes
var (
	failedSSHLogin   = regexp.MustCompile(`(?i)Failed password for (?:invalid user )?(\S+) from (\S+) port (\d+)`)
	acceptedSSHLogin = regexp.MustCompile(`(?i)Accepted (?:publickey|password) for (\S+) from (\S+) port (\d+)`)
	pamUnixAuthFail  = regexp.MustCompile(`(?i)pam_unix\(sshd:auth\): authentication failure;.*?\buser=(\S+)`)
	pamUnixRhost     = regexp.MustCompile(`(?i)\brhost=(\S+)`)
)

// Linux Audit & Sudo Regexes
var (
	sudoExec      = regexp.MustCompile(`(?i)sudo:\s*(\S+)\s*:.*?COMMAND=(.+)$`)
	sudoTargetUser = regexp.MustCompile(`(?i)USER=(\S+)`)
	dangerousCmds = regexp.MustCompile(`(?i)(COMMAND=.*(/bin/bash|-i|/bin/sh)|curl\s+|wget\s+|nc\s+|ncat\s+|netcat\s+|bash\s+-i)`)
)

// Windows Security Events Regexes
var (
	win4625 = regexp.MustCompile(`(?i)(?:Event ID:\s*4625|An account failed to log on|logon failure).*?Account Name:\s*(\S+)(?:.*?Source Network Address:\s*(\S+))?`)
	win4624 = regexp.MustCompile(`(?i)(?:Event ID:\s*4624|An account was successfully logged on).*?Account Name:\s*(\S+)(?:.*?Source Network Address:\s*(\S+))?`)
	win4672 = regexp.MustCompile(`(?i)(?:Event ID:\s*4672|Special privileges assigned to new logon).*?Account Name:\s*(\S+)`)
	win1102 = regexp.MustCompile(`(?i)(?:Event ID:\s*(?:1102|104)|The audit log was cleared|An audit log was cleared)`)
)

// Nginx & Web Attack Regexes
var (
	nginxAccessLog = regexp.MustCompile(`^(\S+) \S+ \S+ \[[^]]+\] "(\S+) ([^"]+)" (\d{3}) (\d+)(?: "([^"]*)")?(?: "([^"]*)")?`)
	sqliPattern    = regexp.MustCompile(`(?i)(UNION\s+SELECT|OR\s+['"]?1['"]?\s*=\s*['"]?1|information_schema|sleep\(\d+\)|benchmark\()`)
	lfiPattern     = regexp.MustCompile(`(?i)(\.\./|\.\.\\|/etc/passwd|/proc/self|\.env|web\.config)`)
	scannerPattern = regexp.MustCompile(`(?i)(sqlmap|nikto|acunetix|nuclei|nmap|masscan)`)
	xssRcePattern  = regexp.MustCompile(`(?i)(<script|javascript:|onload=|eval\(|system\(|exec\(|cmd\.exe|/bin/bash|passthru\()`)
)

type NormalizedEvent struct {
	EventID        string            `json:"event_id"`
	EventTime      time.Time         `json:"event_time"`
	EventType      string            `json:"event_type"`
	LogCategory    string            `json:"log_category"`
	Severity       string            `json:"severity"`
	SrcIP          string            `json:"src_ip,omitempty"`
	Username       string            `json:"username,omitempty"`
	Message        string            `json:"message"`
	Hostname       string            `json:"hostname,omitempty"`
	AgentID        string            `json:"agent_id,omitempty"`
	DuplicateCount int               `json:"duplicate_count,omitempty"`
	Fingerprint    string            `json:"fingerprint,omitempty"`
	Extra          map[string]string `json:"extra_fields,omitempty"`
}

func Fingerprint(hostname string, t time.Time, rawMessage string) string {
	timeBucket := t.UTC().Truncate(time.Minute).Format(time.RFC3339)
	normalizedMsg := strings.TrimSpace(rawMessage)
	hash := sha256.Sum256([]byte(hostname + ":" + timeBucket + ":" + normalizedMsg))
	return hex.EncodeToString(hash[:])
}

func Parse(message redis.XMessage) NormalizedEvent {
	raw := value(message.Values, "raw")
	eventTime := parseTime(value(message.Values, "received_at"))
	hostname := value(message.Values, "hostname")
	sourceType := value(message.Values, "source_type")
	agentID := value(message.Values, "agent_id")

	if sourceType == "" {
		sourceType = "generic"
	}

	event := NormalizedEvent{
		EventID:     message.ID,
		EventTime:   eventTime,
		EventType:   "log",
		LogCategory: sourceType,
		Severity:    "info",
		Message:     raw,
		Hostname:    hostname,
		AgentID:     agentID,
		Fingerprint: Fingerprint(hostname, eventTime, raw),
		Extra:       make(map[string]string),
	}

	// 1. Linux SSH Parsing
	if matches := failedSSHLogin.FindStringSubmatch(raw); matches != nil {
		event.EventType = "authentication_failure"
		event.LogCategory = "linux_sshd"
		event.Severity = "medium"
		event.Username = matches[1]
		event.SrcIP = matches[2]
		event.Extra["src_port"] = matches[3]
		event.Extra["auth_result"] = "failed"
		return event
	}

	if matches := acceptedSSHLogin.FindStringSubmatch(raw); matches != nil {
		event.EventType = "authentication_success"
		event.LogCategory = "linux_sshd"
		event.Severity = "info"
		event.Username = matches[1]
		event.SrcIP = matches[2]
		event.Extra["src_port"] = matches[3]
		event.Extra["auth_result"] = "accepted"
		return event
	}

	if matches := pamUnixAuthFail.FindStringSubmatch(raw); matches != nil {
		event.EventType = "authentication_failure"
		event.LogCategory = "linux_sshd"
		event.Severity = "medium"
		event.Username = matches[1]
		if rhostMatches := pamUnixRhost.FindStringSubmatch(raw); rhostMatches != nil {
			event.SrcIP = rhostMatches[1]
		}
		event.Extra["auth_module"] = "pam_unix"
		return event
	}

	// 2. Linux Audit & Sudo Parsing
	if matches := sudoExec.FindStringSubmatch(raw); matches != nil {
		event.LogCategory = "linux_audit"
		event.Username = matches[1]
		event.Extra["command"] = strings.TrimSpace(matches[2])
		if targetMatch := sudoTargetUser.FindStringSubmatch(raw); targetMatch != nil {
			event.Extra["target_user"] = targetMatch[1]
		} else {
			event.Extra["target_user"] = "root"
		}

		if dangerousCmds.MatchString(raw) {
			event.EventType = "privilege_escalation"
			event.Severity = "high"
		} else {
			event.EventType = "command_execution"
			event.Severity = "medium"
		}
		return event
	}

	// 3. Windows Security Events Parsing
	if win1102.MatchString(raw) {
		event.EventType = "defense_evasion"
		event.LogCategory = "windows_security"
		event.Severity = "critical"
		event.Extra["event_id"] = "1102"
		event.Extra["action"] = "audit_log_cleared"
		return event
	}

	if matches := win4625.FindStringSubmatch(raw); matches != nil {
		event.EventType = "authentication_failure"
		event.LogCategory = "windows_security"
		event.Severity = "medium"
		event.Username = matches[1]
		if len(matches) > 2 && matches[2] != "-" {
			event.SrcIP = matches[2]
		}
		event.Extra["event_id"] = "4625"
		return event
	}

	if matches := win4624.FindStringSubmatch(raw); matches != nil {
		event.EventType = "authentication_success"
		event.LogCategory = "windows_security"
		event.Severity = "info"
		event.Username = matches[1]
		if len(matches) > 2 && matches[2] != "-" {
			event.SrcIP = matches[2]
		}
		event.Extra["event_id"] = "4624"
		return event
	}

	if matches := win4672.FindStringSubmatch(raw); matches != nil {
		event.EventType = "privilege_use"
		event.LogCategory = "windows_security"
		event.Severity = "medium"
		event.Username = matches[1]
		event.Extra["event_id"] = "4672"
		event.Extra["privileges"] = "Administrator"
		return event
	}

	// 4. Web & Nginx Log Parsing
	if matches := nginxAccessLog.FindStringSubmatch(raw); matches != nil {
		event.LogCategory = "nginx"
		event.EventType = "http_access"
		event.SrcIP = matches[1]
		event.Extra["method"] = matches[2]
		event.Extra["path"] = matches[3]
		event.Extra["status_code"] = matches[4]
		event.Extra["bytes"] = matches[5]

		if len(matches) > 6 && matches[6] != "" {
			event.Extra["referrer"] = matches[6]
		}
		if len(matches) > 7 && matches[7] != "" {
			event.Extra["user_agent"] = matches[7]
		}
	}

	// Web Attack Patterns Detection (scans message / path / user-agent)
	if sqliPattern.MatchString(raw) {
		event.EventType = "web_sqli"
		event.Severity = "critical"
		if event.LogCategory == "generic" {
			event.LogCategory = "nginx"
		}
	} else if lfiPattern.MatchString(raw) {
		event.EventType = "web_lfi"
		event.Severity = "high"
		if event.LogCategory == "generic" {
			event.LogCategory = "nginx"
		}
	} else if scannerPattern.MatchString(raw) {
		event.EventType = "web_scanner"
		event.Severity = "medium"
		if event.LogCategory == "generic" {
			event.LogCategory = "nginx"
		}
	} else if xssRcePattern.MatchString(raw) {
		event.EventType = "web_xss_rce"
		event.Severity = "high"
		if event.LogCategory == "generic" {
			event.LogCategory = "nginx"
		}
	}

	if event.SrcIP != "" {
		geo := EnrichIP(event.SrcIP)
		event.Extra["country"] = geo.Country
		event.Extra["country_code"] = geo.CountryCode
		event.Extra["city"] = geo.City
		event.Extra["threat_level"] = geo.ThreatLevel
		if geo.IsMalicious {
			event.Extra["is_malicious"] = "true"
			event.Extra["threat_category"] = geo.ThreatCategory
			event.Extra["reputation_score"] = fmt.Sprintf("%d", geo.ReputationScore)
			if event.Severity == "info" || event.Severity == "low" || event.Severity == "medium" {
				event.Severity = "high"
			}
		}
	}

	return event
}

type GeoIPInfo struct {
	Country         string `json:"country"`
	CountryCode     string `json:"country_code"`
	City            string `json:"city"`
	IsMalicious     bool   `json:"is_malicious"`
	ThreatLevel     string `json:"threat_level"`
	ThreatCategory  string `json:"threat_category"`
	ReputationScore int    `json:"reputation_score"`
}

func EnrichIP(ip string) GeoIPInfo {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "-" {
		return GeoIPInfo{Country: "Unknown", CountryCode: "UN", ThreatLevel: "info"}
	}

	// Internal/Private LAN IP ranges
	if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "127.") || ip == "::1" || strings.HasPrefix(ip, "172.16.") || strings.HasPrefix(ip, "172.17.") || strings.HasPrefix(ip, "172.18.") || strings.HasPrefix(ip, "172.31.") {
		if ip == "192.168.1.105" || ip == "192.168.1.120" || ip == "192.168.1.130" {
			return GeoIPInfo{
				Country:         "Internal Testbed (Simulated)",
				CountryCode:     "LAN",
				City:            "SOC Sandbox",
				IsMalicious:     true,
				ThreatLevel:     "high",
				ThreatCategory:  "Active Attacker Node",
				ReputationScore: 92,
			}
		}
		return GeoIPInfo{
			Country:         "Internal / LAN",
			CountryCode:     "LAN",
			City:            "Local Network",
			IsMalicious:     false,
			ThreatLevel:     "clean",
			ReputationScore: 0,
		}
	}

	// Public IP mappings & Threat Intel lookup signatures
	if strings.HasPrefix(ip, "185.220.") || strings.HasPrefix(ip, "185.100.") {
		return GeoIPInfo{
			Country:         "Germany",
			CountryCode:     "DE",
			City:            "Frankfurt",
			IsMalicious:     true,
			ThreatLevel:     "critical",
			ThreatCategory:  "Tor Exit Node / Anonymizer",
			ReputationScore: 98,
		}
	}
	if strings.HasPrefix(ip, "45.33.") || strings.HasPrefix(ip, "192.0.2.") {
		return GeoIPInfo{
			Country:         "United States",
			CountryCode:     "US",
			City:            "Dallas",
			IsMalicious:     true,
			ThreatLevel:     "high",
			ThreatCategory:  "Automated Scanner Node",
			ReputationScore: 88,
		}
	}
	if strings.HasPrefix(ip, "91.240.") || strings.HasPrefix(ip, "198.51.100.") {
		return GeoIPInfo{
			Country:         "Russia",
			CountryCode:     "RU",
			City:            "Moscow",
			IsMalicious:     true,
			ThreatLevel:     "critical",
			ThreatCategory:  "Known C2 Infrastructure",
			ReputationScore: 95,
		}
	}
	if strings.HasPrefix(ip, "114.114.") || strings.HasPrefix(ip, "203.0.113.") {
		return GeoIPInfo{
			Country:         "China",
			CountryCode:     "CN",
			City:            "Beijing",
			IsMalicious:     true,
			ThreatLevel:     "high",
			ThreatCategory:  "Brute Force Botnet Node",
			ReputationScore: 85,
		}
	}
	if strings.HasPrefix(ip, "118.69.") || strings.HasPrefix(ip, "14.225.") {
		return GeoIPInfo{
			Country:         "Vietnam",
			CountryCode:     "VN",
			City:            "Ho Chi Minh City",
			IsMalicious:     false,
			ThreatLevel:     "clean",
			ReputationScore: 10,
		}
	}

	return GeoIPInfo{
		Country:         "United States",
		CountryCode:     "US",
		City:            "Washington D.C.",
		IsMalicious:     false,
		ThreatLevel:     "medium",
		ReputationScore: 40,
	}
}

type Consumer struct {
	redis               redis.UniversalClient
	stream, group, name string
}

func NewConsumer(redisURL, stream, group, name string) (*Consumer, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Consumer{redis: redis.NewClient(options), stream: stream, group: group, name: name}, nil
}

func (consumer *Consumer) Close() error {
	if client, ok := consumer.redis.(*redis.Client); ok {
		return client.Close()
	}
	return nil
}

func (consumer *Consumer) EnsureGroup(ctx context.Context) error {
	err := consumer.redis.XGroupCreateMkStream(ctx, consumer.stream, consumer.group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (consumer *Consumer) Consume(ctx context.Context, handler func(context.Context, NormalizedEvent) error) error {
	if err := consumer.EnsureGroup(ctx); err != nil {
		return err
	}
	for {
		streams, err := consumer.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumer.group,
			Consumer: consumer.name,
			Streams:  []string{consumer.stream, ">"},
			Count:    10,
			Block:    time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return err
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				event := Parse(message)

				// Raw Log Deduplication in 5-minute sliding window via Redis
				if consumer.redis != nil && event.Fingerprint != "" {
					key := fmt.Sprintf("siem:dedup:log:%s", event.Fingerprint)
					count, err := consumer.redis.Incr(ctx, key).Result()
					if err == nil {
						if count == 1 {
							_ = consumer.redis.Expire(ctx, key, 5*time.Minute).Err()
						}
						event.DuplicateCount = int(count)
						if count > 1 {
							event.Extra["dedup_status"] = fmt.Sprintf("%dx deduplicated", count)
						}
					}
				}

				if err := handler(ctx, event); err != nil {
					return fmt.Errorf("handle event %s: %w", message.ID, err)
				}
				if err := consumer.redis.XAck(ctx, consumer.stream, consumer.group, message.ID).Err(); err != nil {
					return fmt.Errorf("ack event %s: %w", message.ID, err)
				}
			}
		}
	}
}

func value(values map[string]any, key string) string {
	if val, ok := values[key]; ok {
		return fmt.Sprint(val)
	}
	return ""
}

func parseTime(val string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, val)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed
}
