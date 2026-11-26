#!/bin/sh
# Writes the runtime configuration for the HealthHub web app before nginx
# starts. The official nginx image runs every script in /docker-entrypoint.d.
#
#   API_BASE_URL  Origin of the HealthHub API, for example
#                 http://192.168.1.96:8081. Unset or empty means the API is
#                 served from the same origin as the app (behind a proxy).
set -eu

out=/tmp/healthhub
mkdir -p "$out"

# Trim surrounding whitespace and trailing slashes.
url=$(printf '%s' "${API_BASE_URL:-}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's:/*$::')

# Accept only a plain http(s) origin with an optional path. Anything else is
# refused rather than escaped: the value is written into JavaScript and into
# the Content-Security-Policy header.
if [ -n "$url" ] && ! printf '%s' "$url" | grep -Eq '^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~/-]*)?$'; then
    echo "40-healthhub-config: API_BASE_URL must look like http://host:port or https://host/path" >&2
    exit 1
fi

origin=""
if [ -n "$url" ]; then
    origin=$(printf '%s' "$url" | sed -E 's#^(https?://[^/]+).*#\1#')
fi

cat > "$out/config.js" <<EOF
// Generated at container start from API_BASE_URL.
window.__HEALTHHUB_CONFIG__ = { apiBaseUrl: "$url" };
EOF

connect_src="'self'"
if [ -n "$origin" ]; then connect_src="'self' $origin"; fi

cat > "$out/headers.conf" <<EOF
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src $connect_src; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "DENY" always;
add_header Referrer-Policy "no-referrer" always;
add_header Cross-Origin-Opener-Policy "same-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
EOF

if [ -n "$url" ]; then
    echo "40-healthhub-config: API base URL is $url"
else
    echo "40-healthhub-config: API base URL is the same origin"
fi
