# Istio CA rotation

## Introduction

This is a controller for rotating Istio root CA. See https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/ for information abou the process. The controller does the rotation in two parts:

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
