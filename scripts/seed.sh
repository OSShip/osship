#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! docker compose ps postgres --status running -q 2>/dev/null | grep -q .; then
  echo "Postgres is not running. Start the stack with: make up"
  exit 1
fi

echo "==> Seeding demo data for payments flow"

IFS=$'\t' read -r DEMO_SALT DEMO_HASH < <(go run ./utils/passhash/cmd/seedhash/main.go password123)

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U osship -d osship <<SQL
SET search_path TO general;

-- Demo users (password: password123, per-user random salt + bcrypt)
INSERT INTO users (id, email, password_hash, password_salt, role, github_username, display_name)
VALUES
  ('11111111-1111-1111-1111-111111111101', 'student@osship.local', '$DEMO_HASH', '$DEMO_SALT', 'student', 'demo-student', 'Demo Student'),
  ('11111111-1111-1111-1111-111111111102', 'mentor@osship.local', '$DEMO_HASH', '$DEMO_SALT', 'mentor', 'demo-mentor', 'Demo Mentor'),
  ('11111111-1111-1111-1111-111111111103', 'admin@osship.local', '$DEMO_HASH', '$DEMO_SALT', 'admin', 'demo-admin', 'Demo Admin')
ON CONFLICT (email) DO UPDATE SET
  password_hash = EXCLUDED.password_hash,
  password_salt = EXCLUDED.password_salt;

INSERT INTO mentor_applications (user_id, status, github_data)
SELECT '11111111-1111-1111-1111-111111111102', 'approved', '{"summary":{"login":"demo-mentor","public_repos":12}}'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM mentor_applications WHERE user_id = '11111111-1111-1111-1111-111111111102');

INSERT INTO listings (id, mentor_id, oss_project_name, oss_repo_url, description, price_cents, duration_weeks, total_slots, status)
VALUES (
  '22222222-2222-2222-2222-222222222201',
  '11111111-1111-1111-1111-111111111102',
  'React Core Mentorship',
  'https://github.com/facebook/react',
  'Structured mentorship contributing to React. Weekly sessions, PR reviews, and OSS workflow guidance.',
  9900,
  8,
  5,
  'active'
)
ON CONFLICT (id) DO NOTHING;
SQL

echo "Demo accounts (see accounts.txt):"
echo "  student@osship.local / password123"
echo "  mentor@osship.local  / password123"
echo "  admin@osship.local   / password123"
echo "Listing ID: 22222222-2222-2222-2222-222222222201"
