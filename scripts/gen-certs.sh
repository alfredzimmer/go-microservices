#!/usr/bin/env bash
# Generates a local CA and one identity certificate per service for mutual TLS.
# Certs land in certs/ (git-ignored). Idempotent: skips work if certs exist;
# use `make certs-clean` to force regeneration.
set -euo pipefail

cd "$(dirname "$0")/.."
CERT_DIR=certs
SERVICES=(account catalog order graphql)
DAYS=3650

if [[ -f "$CERT_DIR/ca.crt" ]]; then
  echo "certs already present in $CERT_DIR/ (run 'make certs-clean' to regenerate)"
  exit 0
fi

mkdir -p "$CERT_DIR"

openssl genrsa -out "$CERT_DIR/ca.key" 4096 2>/dev/null
openssl req -x509 -new -key "$CERT_DIR/ca.key" -sha256 -days "$DAYS" \
  -subj "/CN=go-microservices-local-ca" \
  -out "$CERT_DIR/ca.crt"

for svc in "${SERVICES[@]}"; do
  openssl genrsa -out "$CERT_DIR/$svc.key" 2048 2>/dev/null
  openssl req -new -key "$CERT_DIR/$svc.key" \
    -subj "/CN=$svc" \
    -out "$CERT_DIR/$svc.csr"
  # Every service acts as both a TLS server and a TLS client (mTLS), so each
  # cert carries both EKUs. SANs cover the compose DNS name and host access.
  openssl x509 -req -in "$CERT_DIR/$svc.csr" \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
    -days "$DAYS" -sha256 \
    -extfile <(printf '%s\n' \
      "subjectAltName=DNS:$svc,DNS:localhost,IP:127.0.0.1" \
      "extendedKeyUsage=serverAuth,clientAuth" \
      "keyUsage=digitalSignature,keyEncipherment" \
      "basicConstraints=CA:FALSE") \
    -out "$CERT_DIR/$svc.crt" 2>/dev/null
  rm "$CERT_DIR/$svc.csr"
done
rm -f "$CERT_DIR/ca.srl"

echo "generated CA and certs for: ${SERVICES[*]} -> $CERT_DIR/"
