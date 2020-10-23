openssl genrsa -out root-key.pem 2048
openssl req -x509 -new -nodes -key root-key.pem -sha256 -days 1825 -out root-cert.pem

openssl genrsa -out ca-key.pem 2048
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 1825 -out ca-cert.pem

cp ca-cert.pem cert-chain.pem

kubectl create secret generic new-secret -n istio-system --from-file=ca-cert.pem --from-file=ca-key.pem --from-file=root-cert.pem --from-file=cert-chain.pem
