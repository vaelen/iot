#!/bin/sh

# Generate keys for a Google IoT Core device.

openssl genrsa -out rsa_private.pem 2048
openssl req -x509 -nodes -newkey rsa:2048 -keyout rsa_private.pem -days 1000000 -out rsa_cert.pem -subj "/CN=unused"

openssl ecparam -genkey -name prime256v1 -noout -out ec_private.pem
openssl req -x509 -new -key ec_private.pem -days 1000000 -out ec_cert.pem -subj "/CN=unused"
