# Istio CA rotation

## Obsolete

Intermediate CA rotation is supported in upstream. We are making
this project obsolete.

This feature is supported in upstream (https://github.com/istio/istio/pull/31522).
It is disabled by default in Istio. To utilize this feature please enable
it through environment variable AUTO_RELOAD_PLUGIN_CERTS. This avoids restarting
istiod when new Intermediate CA is introduced. Istiod will monitor the CA files
and automatically loads the certs when it notice the changes. Root CA rotation
is not yet supported in upstream as well. If you would like to introduce new
Root CA, restart istiod and all workloads.


## Introduction

This is a controller for rotating Istio intermediate CA (root CA
rotation will be supported in the future). See
https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/
for information about the process.

  1. Create a new intermediate CA which is based on current root CA.
  2. Install a NewCA CR with a known name which points to the new
     intermediate CA secret.
  3. Controller checks if intermediate CA certificate is changed. It
     installs the new CA cert and key as a plugin-in CA and restarts
     istiod. Then workload certificates will be propagated to workloads.
     Controller will not rotate certs if root cert is changed.
  4. Controller reports errors and conditions back in the Status field
     of the NewCA object.

Future work (If root CA has been changed):

  1. Create a combined CA secret (old and new CA certificates together)
     and install it. Wait until it has propagated to the workloads. The
     combined certitifate means that the workloads will be able to
     authenticate mTLS connections whether or not the other end of the
     connection has a ceritificate already signed by the new CA key.
  2. When all workloads have updated workload certs, the CA secret is
     updated to contain only the new intermediate certificate.

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
