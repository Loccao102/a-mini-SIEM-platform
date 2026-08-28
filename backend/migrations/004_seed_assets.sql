-- Seed demo assets for development/demo purposes
INSERT INTO assets (hostname, ip_address, os_type, criticality, owner, tags)
VALUES
  ('web-prod-01',    '192.168.1.10',  'linux',   'critical', 'SRE Team',       '{"role":"web","env":"production"}'),
  ('db-prod-01',     '192.168.1.20',  'linux',   'critical', 'DBA Team',        '{"role":"database","env":"production"}'),
  ('win-dc-01',      '192.168.1.30',  'windows', 'critical', 'IT Admin',        '{"role":"domain-controller","env":"production"}'),
  ('web-staging-01', '192.168.1.40',  'linux',   'high',     'Dev Team',        '{"role":"web","env":"staging"}'),
  ('desktop-dev-01', '192.168.1.50',  'windows', 'medium',   'Dev Team',        '{"role":"workstation","env":"dev"}'),
  ('monitoring-01',  '192.168.1.60',  'linux',   'high',     'SRE Team',        '{"role":"monitoring","env":"production"}'),
  ('DESKTOP-OVVPOR2','192.168.1.100', 'windows', 'medium',   NULL,              '{"role":"workstation","env":"dev"}')
ON CONFLICT DO NOTHING;

-- Seed demo log sources linked to each asset
INSERT INTO log_sources (asset_id, source_type, agent_id, status, last_seen)
SELECT asset_id, 'linux_sshd',      'filebeat-' || asset_id, 'active', now() FROM assets WHERE os_type = 'linux'
ON CONFLICT DO NOTHING;

INSERT INTO log_sources (asset_id, source_type, agent_id, status, last_seen)
SELECT asset_id, 'windows_eventlog', 'winlogbeat-' || asset_id, 'active', now() FROM assets WHERE os_type = 'windows'
ON CONFLICT DO NOTHING;
