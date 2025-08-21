#!/bin/bash

# This script generates a self-signed CA certificate, a server certificate, and a client certificate.
# It is intended for use with RabbitMQ and TLS/SSL configurations for local development and testing.
# It requires OpenSSL to be installed on the system.
# Usage: Run this script in the terminal. It will create a 'certs' directory with the generated certificates.

set -e

echo Generating CA certificate and private key...
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -config ca.cnf

echo ""
echo Generating server certificate and private key...
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -config server.csr.cnf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -sha256 -extfile server.ext

echo ""
echo Generating client certificate and private key...
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -config client.csr.cnf
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256 -extfile client.ext

mkdir -p ./certs
mv ca.crt ./certs/ca.crt
mv server.crt ./certs/server.crt
mv server.key ./certs/server.key
mv client.crt ./certs/client.crt
mv client.key ./certs/client.key
chmod 644 ./certs/*