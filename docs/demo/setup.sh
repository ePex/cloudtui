#!/usr/bin/env bash
# Builds the "example" instance the demo GIFs are recorded on:
#   - a throwaway ActiveMQ broker in a container (ports 18161/16616, so it
#     never clashes with a dev broker on 8161/61616)
#   - generic example queues and messages (no real data)
#   - a separate HOME under /tmp/cloudtui-demo with a single connection
#     called "example" and the repo's example snippets
#
# Usage (from the repo root): docs/demo/setup.sh        # set up
#                             docs/demo/setup.sh --down # remove it all
# Needs podman or docker, Go, and python3. POSIX shells only.
set -euo pipefail

DEMO=/tmp/cloudtui-demo
CONTAINER=cloudtui-demo-amq
JOLOKIA=http://localhost:18161/api/jolokia
REPO=$(cd "$(dirname "$0")/../.." && pwd)

runtime=$(command -v podman || command -v docker || true)
[ -n "$runtime" ] || { echo "needs podman or docker" >&2; exit 1; }

if [ "${1:-}" = "--down" ]; then
  "$runtime" rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$DEMO"
  echo "demo instance removed"
  exit 0
fi

# --- broker ---------------------------------------------------------------
"$runtime" rm -f "$CONTAINER" >/dev/null 2>&1 || true
"$runtime" run -d --name "$CONTAINER" -p 18161:8161 -p 16616:61616 \
  docker.io/apache/activemq-classic:latest >/dev/null
for _ in $(seq 1 60); do
  curl -sf -u admin:admin -H "Origin: http://localhost:18161" "$JOLOKIA/version" >/dev/null && break
  sleep 2
done

# --- app and HOME ---------------------------------------------------------
rm -rf "$DEMO"
mkdir -p "$DEMO/home/.cloudtui/connections" "$DEMO/bin" "$DEMO/files"
(cd "$REPO/tui" && go build -o "$DEMO/bin/cloudtui" ./cmd/cloudtui && go build -o "$DEMO/bin/devtool" ./cmd/devtool)
cat > "$DEMO/home/.cloudtui/connections/jolokia.yaml" <<EOF
- name: example
  backend: jolokia
  queue:
    brokerName: localhost
    url: $JOLOKIA
    username: admin
    password: admin
EOF
printf 'activeConnection: example\ntheme: dark\n' > "$DEMO/home/.cloudtui/config.yaml"
cp -R "$REPO/examples/snippets" "$DEMO/home/.cloudtui/snippets"

# A plain JSON file outside the library, for the import demo.
printf '{"orderId":"ORD-1042","customer":"initech","items":[{"sku":"SKU-003","quantity":4}],"total":88.40}' \
  > "$DEMO/files/order-ORD-1042.json"

# --- queues and messages --------------------------------------------------
for q in orders.created orders.shipped payments.received dlq.orders.created inventory.updates notifications.email; do
  (cd "$DEMO" && HOME="$DEMO/home" ./bin/devtool add-queue "$q" >/dev/null)
done

send() { # queue jmsType correlationID body
  python3 - "$@" <<'PY'
import base64, json, sys, urllib.request
queue, jms_type, corr, body = sys.argv[1:5]
headers = {"JMSType": jms_type}
if corr:
    headers["JMSCorrelationID"] = corr
req = {
    "type": "exec",
    "mbean": f"org.apache.activemq:type=Broker,brokerName=localhost,destinationType=Queue,destinationName={queue}",
    "operation": "sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)",
    "arguments": [headers, body, "admin", "admin"],
}
r = urllib.request.Request(
    "http://localhost:18161/api/jolokia/", data=json.dumps(req).encode(),
    headers={"Content-Type": "application/json", "Origin": "http://localhost:18161",
             "Authorization": "Basic " + base64.b64encode(b"admin:admin").decode()})
assert json.load(urllib.request.urlopen(r))["status"] == 200
PY
}

customers=(acme-corp globex initech umbrella soylent)
for i in $(seq 1 9); do
  send orders.created OrderCreated "ord-$((1000 + i))" \
    "{\"orderId\":\"ORD-$((1000 + i))\",\"customer\":\"${customers[$((i % 5))]}\",\"items\":[{\"sku\":\"SKU-00$((i % 4 + 1))\",\"quantity\":$((i % 3 + 1))}],\"total\":$((i * 7 + 12)).90}"
done
for i in 1 2 3 4; do
  send orders.shipped OrderShipped "ord-$((1000 + i))" \
    "{\"orderId\":\"ORD-$((1000 + i))\",\"carrier\":\"example-express\",\"trackingNo\":\"TRK-$((5000 + i))\"}"
done
for i in 1 2 3 4 5 6; do
  send payments.received PaymentReceived "ord-$((1000 + i))" \
    "<payment><paymentId>PAY-$((2000 + i))</paymentId><orderId>ORD-$((1000 + i))</orderId><amount currency=\"EUR\">$((i * 7 + 12)).90</amount></payment>"
done
for i in 7 8 9; do
  send dlq.orders.created OrderCreated "ord-$((1000 + i))" \
    "{\"orderId\":\"ORD-$((1000 + i))\",\"customer\":\"globex\",\"error\":\"inventory service timeout\"}"
done
for i in 1 2 3; do
  send inventory.updates StockUpdated "" "{\"sku\":\"SKU-00$i\",\"warehouse\":\"WH-EAST\",\"quantity\":$((100 + i * 9))}"
done

echo "demo instance ready: HOME=$DEMO/home $DEMO/bin/cloudtui"
