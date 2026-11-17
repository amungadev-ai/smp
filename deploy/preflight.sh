#!/usr/bin/env bash
# M10 read-only preflight; never launches Docker, Caddy or live Go engine.
set -Eeuo pipefail
mode=''
domain=''
expected=''
ack=0
pass(){ echo "PASS $*"; }
fail(){ echo "BLOCKED $*" >&2;exit 2; }
usage(){
 cat <<'EOF'
Drevanta M10 preflight (read-only):
  ./deploy/preflight.sh --plan
  ./deploy/preflight.sh --check-local
  ./deploy/preflight.sh --check-live --domain app.your-test-domain.example \
      --expected-ip PUBLIC_IPV4 --confirm-disposable

Live preflight only checks prerequisites and an unused dedicated Caddy instance.
It never deploys workloads, changes DNS, opens ports, or enables --live.
Passing it does NOT mean production is approved.
EOF
}
while (($#));do
 case "$1" in
  --plan|--check-local|--check-live) [[ -z "$mode" ]] || fail 'Choose exactly one mode';mode="$1";;
  --domain) shift;[[ $# -gt 0 ]] || fail 'Missing domain';domain="$1";;
  --expected-ip) shift;[[ $# -gt 0 ]] || fail 'Missing expected IP';expected="$1";;
  --confirm-disposable) ack=1;;
  --help|-h)usage;exit 0;;
  *)fail "Unknown option $1";;
 esac
 shift
done
[[ -n "$mode" ]] || { usage;exit 2; }
if [[ "$mode" == --plan ]];then
 echo 'Read-only preflight: platform + Go + Node checks; no host changes.'
 echo 'For a disposable live VPS, also inspect dedicated Caddy, Docker and DNS.'
 echo 'NO live deployment or automatic HTTPS issuance occurs.'
 exit 0
fi
[[ "$(uname -s)" == Linux ]] || fail 'Linux required'
case "$(uname -m)" in x86_64|aarch64) ;; *)fail 'Only x86_64/arm64 supported';; esac
for c in go node npm curl systemctl flock;do command -v "$c" >/dev/null 2>&1 || fail "Missing $c";done
node -e 'process.exit(Number(process.versions.node.split(".")[0])>=22?0:1)' || fail 'Node 22+ required'
gv="$(go env GOVERSION)"
if [[ "$gv" =~ ^go([0-9]+)[.]([0-9]+) ]];then
 (( ${BASH_REMATCH[1]} > 1 || ( ${BASH_REMATCH[1]} == 1 && ${BASH_REMATCH[2]} >= 22 ) )) || fail 'Go 1.22+ required'
else fail 'Unable to determine Go version';fi
pass "Linux platform, Go $gv, Node $(node -v), npm $(npm -v)"
if [[ "$mode" == --check-local ]];then
 echo 'Use the M09 installer in mock-only mode. Check services with deploy/verify.sh after installation.'
 exit 0
fi
# Never contact Docker or Caddy without explicit disposable-VPS confirmation.
[[ "$ack" == 1 ]] || fail 'Disposable VPS acknowledgment required'
[[ ${#domain} -le 253 && "$domain" != *..* ]] || fail 'Provide a valid dedicated test hostname'
IFS='.' read -r -a labels <<<"$domain"
(( ${#labels[@]} >= 2 )) || fail 'Provide a valid dedicated test hostname'
for label in "${labels[@]}";do
 [[ "$label" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ ]] || fail 'Provide a valid dedicated test hostname'
done
tld="${labels[${#labels[@]}-1]}"
[[ "$tld" =~ ^[a-z]{2,24}$ ]] || fail 'Provide a valid dedicated test hostname'
[[ "$expected" =~ ^[0-9]{1,3}([.][0-9]{1,3}){3}$ ]] || fail 'Supply a dotted public IPv4 address'
IFS='.' read -r a b c d <<<"$expected"
for n in "$a" "$b" "$c" "$d";do
 [[ "$n" =~ ^[0-9]{1,3}$ ]] || fail 'Invalid IPv4 octet'
 (( 10#$n <= 255 )) || fail 'IPv4 octet exceeds 255'
done
a_num=$((10#$a));b_num=$((10#$b));c_num=$((10#$c))
if (( a_num==0 || a_num==10 || a_num==127 || a_num>=224 ||
       (a_num==172 && b_num>=16 && b_num<=31) ||
       (a_num==192 && (b_num==168 || (b_num==0 && c_num==2))) ||
       (a_num==169 && b_num==254) ||
       (a_num==100 && b_num>=64 && b_num<=127) ||
       (a_num==198 && (b_num==18 || b_num==19 || b_num==51)) ||
       (a_num==203 && b_num==0 && c_num==113) ));then
 fail 'Expected IP is private, reserved or non-public'
fi
for c in docker jq getent;do command -v "$c" >/dev/null 2>&1 || fail "Missing $c";done
[[ -S /var/run/docker.sock ]] || fail 'Docker socket not present'
docker info --format '{{.ServerVersion}}' >/dev/null 2>&1 || fail 'Docker daemon not accessible'
pass 'Docker daemon responds (read-only)'
getent ahostsv4 "$domain" | awk '{print $1}' | grep -Fxq "$expected" || fail "Test DNS $domain does not resolve to $expected"
pass "DNS resolves to $expected"
# Caddy GET /config/ is read-only. We reject any existing HTTP routes.
cfg="$(curl --fail-with-body -fsS --connect-timeout 2 --max-time 5 --max-filesize 262144 http://127.0.0.1:2021/config/)" || fail 'Dedicated Caddy API missing on 127.0.0.1:2021'
printf '%s' "$cfg" | jq -e '(.admin.listen=="127.0.0.1:2021") and (([.apps.http.servers[]?.routes[]?]|length)==0)' >/dev/null || fail 'Dedicated Caddy has routes or wrong admin endpoint; refusing shared instance'
pass 'Dedicated Caddy API has the expected admin listener and no existing HTTP routes'
echo 'Read-only checks passed. NOT VERIFIED: public TLS, ports 80/443, network isolation, rollback under load, and off-host recovery.'
echo 'Do not enable production deployments. Perform M08C disposable live smoke and document results.'
