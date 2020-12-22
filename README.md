# Istio CA rotation

## Introduction

This is a controller for rotating Istio intermediate CA (ROOT CA rotation will be supported in future). See https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/ for information abou the process.

  1. Create a new intermediate CA which is based on current ROOT CA.
  2. Controller checks if intermediate certs are changed, it will install new certs and start istiod to take new certs. Then certs will be propagated to worklods.
  3. Controller will ignore if root certs are changed.

Future work (If ROOT-CA has been changed):
  1. Create a combined CA secret (old and new CA certificates together) and install it. Wait until it has propagated to the workloads. The combined certitifate means that the workloads will be able to authenticate mTLS connections whether or not the other end of the connection has a ceritificate already signed by the new CA key.
  2. When all workloads have updated workload certs, the CA secret is updated to contain only the new intermediate certificate.

## Installation

    # Build the container
    make docker-build IMG=istio-ca-rotation-controller
    
    # Tag and push it to the registry
    docker tag istio-ca-rotation-controller <registry-name>/<tag>
    make docker-push IMG=<registry-name>/<tag>
    
    # Deploy it to the cluster
    make deploy IMG=<registry-name>/<tag>

## Testing

    # Create certs according to instructions in Istio docs:
    # Below scripts are available from istio-1.8.0

    cd <istio-dir>

    # It will generate root-ca
    make -f tools/certs/Makefile.selfsigned.mk root-ca
    # It will generate intermediate certs based on root-ca
    make -f tools/certs/Makefile.selfsigned.mk intermediate1-cacerts
    # It will generate another intermediate certs based on above root-ca
    make -f tools/certs/Makefile.selfsigned.mk intermediate2-cacerts

    # Assuming that cluster should have intermediate1 certs already applied in the system. We are going to rotate intermediate2 certs.

    kubectl create secret generic new-secret -n istio-system --from-file=intermediate2/ca-cert.pem --from-file=intermediate2/ca-key.pem --from-file=intermediate2/root-cert.pem --from-file=intermediate2/cert-chain.pem
    
    # Install the NewCA object
    kubectl apply -f config/samples/istiocarotation_v1_newca.yaml
