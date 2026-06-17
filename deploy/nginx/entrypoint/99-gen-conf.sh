#!/bin/sh
set -eu

domain="${DOMAIN:-}"
upstream="${UPSTREAM:-api:8080}"

fail() {
  echo "$1" >&2
  # The official nginx entrypoint may source scripts that are not executable.
  # Avoid a hard exit when sourced.
  return 1 2>/dev/null || exit 1
}

if [ -z "$domain" ]; then
  fail "DOMAIN is required"
fi

conf_dir="/etc/nginx/conf.d"
conf_file="$conf_dir/default.conf"

mkdir -p "$conf_dir"

cert_fullchain="/etc/letsencrypt/live/$domain/fullchain.pem"
cert_privkey="/etc/letsencrypt/live/$domain/privkey.pem"

# Always serve HTTP for ACME http-01. Redirect everything else to HTTPS.
cat > "$conf_file" <<EOF
server {
  listen 80;
  listen [::]:80;

  server_name ${domain};

  location /.well-known/acme-challenge/ {
    root /var/www/certbot;
  }

  location / {
    return 301 https://\$host\$request_uri;
  }
}
EOF

# Only enable 443 when certificate exists; otherwise nginx would fail to start.
if [ -f "$cert_fullchain" ] && [ -f "$cert_privkey" ]; then
  cat >> "$conf_file" <<EOF

server {
  listen 443 ssl;
  listen [::]:443 ssl;

  server_name ${domain};

  ssl_certificate ${cert_fullchain};
  ssl_certificate_key ${cert_privkey};

  # Basic hardening
  ssl_session_cache shared:SSL:10m;
  ssl_session_timeout 1d;

  # Adjust if upload endpoints need more
  client_max_body_size 50m;

  location / {
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;

    proxy_pass http://${upstream};
  }
}
EOF
fi
