#!/bin/sh
# Run as root on a fresh OpenBSD Vultr VPS.
# Deploys the personal-site Go binary + configures httpd / relayd / acme-client.

set -e

DOMAIN="ridgway.dev"
APP_DIR="/var/www/personal-site"
BINARY="/usr/local/bin/personal-site"

# ── System packages (none needed — all stdlib + single binary) ──────────────
pkg_add -u

# ── App directories ──────────────────────────────────────────────────────────
mkdir -p "$APP_DIR/content/posts"
mkdir -p "$APP_DIR/static/css"
mkdir -p "$APP_DIR/static/images"
mkdir -p "$APP_DIR/templates/admin"
mkdir -p /var/www/acme
chown -R www:www "$APP_DIR"

echo "Directories created."

# ── Copy binary (built on CI or locally) ─────────────────────────────────────
# Build: GOOS=openbsd GOARCH=amd64 go build -o personal-site .
# scp personal-site root@<vps-ip>:/usr/local/bin/personal-site
echo "Place built binary at $BINARY and re-run if needed."

# ── TLS / ACME ────────────────────────────────────────────────────────────────
mkdir -p /etc/acme /etc/ssl/private
chmod 700 /etc/ssl/private

cp deploy/acme-client.conf /etc/acme-client.conf
cp deploy/httpd.conf /etc/httpd.conf

# Enable httpd for ACME challenges first (no TLS yet)
rcctl enable httpd
rcctl start httpd

# Get initial certificate
acme-client -v "$DOMAIN"

# ── relayd ────────────────────────────────────────────────────────────────────
cp deploy/relayd.conf /etc/relayd.conf
rcctl enable relayd
rcctl start relayd

# ── rc.d service ─────────────────────────────────────────────────────────────
cp deploy/personalsite /etc/rc.d/personalsite
chmod 555 /etc/rc.d/personalsite
rcctl enable personalsite
rcctl set personalsite flags ""

# ── Secrets file ─────────────────────────────────────────────────────────────
if [ ! -f /etc/personal-site.env ]; then
    cat > /etc/personal-site.env <<'EOF'
ADMIN_USER=admin
ADMIN_HASH=9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
SESSION_SECRET=CHANGE_ME_RANDOM_STRING
INTERVALS_API_KEY=
INTERVALS_ATHLETE_ID=
EOF
    chmod 600 /etc/personal-site.env
    echo "Edit /etc/personal-site.env and set real secrets before starting."
fi

# ── pf — basic firewall ───────────────────────────────────────────────────────
cat >> /etc/pf.conf <<'EOF'

# Personal site
pass in on egress proto tcp to port { 80 443 } keep state
pass in on egress proto tcp from any to any port 22 keep state
EOF
pfctl -f /etc/pf.conf

# ── cron — renew cert weekly ─────────────────────────────────────────────────
(crontab -l 2>/dev/null; echo "0 3 * * 1 acme-client $DOMAIN && rcctl reload relayd") | crontab -

echo ""
echo "Setup complete. Steps remaining:"
echo "  1. Upload binary:  scp personal-site root@<ip>:$BINARY"
echo "  2. Upload assets:  rsync -r wwwroot/ root@<ip>:$APP_DIR/static/"
echo "  3. Upload posts:   rsync -r Content/posts/ root@<ip>:$APP_DIR/content/posts/"
echo "  4. Upload tmpls:   rsync -r templates/ root@<ip>:$APP_DIR/templates/"
echo "  5. Edit secrets:   vi /etc/personal-site.env"
echo "  6. Start app:      rcctl start personalsite"
