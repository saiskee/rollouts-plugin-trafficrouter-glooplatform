**Code:**
[![Go Report Card](https://goreportcard.com/badge/github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform)](https://goreportcard.com/report/github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform)
[![Gateway API plugin CI](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/actions/workflows/ci.yaml/badge.svg)](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/actions/workflows/ci.yaml)

**Social:**
[![Twitter Follow](https://img.shields.io/twitter/follow/argoproj?style=social)](https://twitter.com/argoproj)
[![Slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)

# Argo Rollout Gloo Platform API Plugin

<img align="right" src="img/logo.png">

An Argo Rollouts plugin for [Gloo Platform](https://www.solo.io/products/gloo-platform/).

### Quickstart

Install Argo Rollouts w/ downloaded Gloo Platform plugin

```bash
kubectl create ns argo-rollouts
kubectl apply -k ./deploy -n argo-rollouts
```

Deploy the example initial state

```bash
kubectl apply -f ./examples/demo-api-initial-state
kubectl apply -f ./examples/0-rollout-initial
```

Create a rollout revision and view in dashboard

```bash
kubectl apply -f ./examples/1-rollout-first-change
kubectl argo rollouts dashboard &
open http://localhost:3100/rollouts
```

### Argo Rollouts Plugin Installation

Requirements:

1. Gloo Platform plugin the Argo Rollouts runtime container
1. Register the plugin in the Argo Rollouts argo-rollouts-config ConfigMap
1. Argo Rollouts RBAC to modify Gloo APIs

The plugin can be loaded into the controller runtime by building your own Argo Rollouts image, pulling it in an init container, or having the controller download it on startup. See [Traffic Router Plugins](https://argoproj.github.io/argo-rollouts/features/traffic-management/plugins/) for details.

See [Kustomize patches](./deploy/kustomization.yaml) in this repo for Argo Rollouts configuration examples.

### Usage

Canary and stable services in the Rollout spec must refer to `forwardTo` destinations in [routes](https://docs.solo.io/gloo-mesh-enterprise/latest/troubleshooting/gloo/routes/) that exist in one or more Gloo Platform RouteTables.

RouteTable and route selection is specified in the plugin config. Either a RouteTable label selector or a named RouteTable must be specified. RouteSelector is entirely optional; this is useful to limit matches to specific routes in a RouteTable if it contains any references to canary or stable services that you do not want to modify.

```yaml
  strategy:
    canary:
      canaryService: canary
      stableService: stable
      trafficRouting:
        plugins:
          # the plugin name must match the name used in argo-rollouts-config ConfigMap
          solo-io/glooplatformAPI:
            # select Gloo RouteTable(s); if both label and name selectors are used, the name selector
            # takes precedence
            routeTableSelector:
              # (optional) label selector
              labels:
                app: demo
              # filter by namespace
              namespace: gloo-mesh
              # (optional) select a specific RouteTable by name
              # name: rt-name
            # (optional) select specific route(s); useful to target specific routes in a RouteTable that has mutliple occurences of the canaryService or stableService 
            routeSelector:
              # (optional) label selector
              labels:
                route: demo-preview
              # (optional) select a specific route by name
              # name: route-name
```

### Examples

1. `kubectl apply -f ./examples/demo-api-initial-state && kubectl apply -f ./examples/0-rollout-initial`
1. Observe the initial rollout; it should have fully deployed the demo api b/c it was the first revision of the Rollout
1. `kubectl apply -f ./examples/1-rollout-first-change`
1. Observe the rollout canaried 10% of traffic to v2 and is now paused
1. Perform the remaining rollout steps until fully promoted
1. `kubectl apply -f ./examples/2-rollout-second-change`
1. Repeat 4 & 5

### TODO

- implement [blue/green](./pkg/plugin/plugin_bluegreen.go)
- implement `SetHeaderRoute` and `SetMirrorRoute` in [plugin.go](./pkg/plugin/plugin.go)
- unit tests
  - update tests with mock gloo client using interfaces in [./pkg/gloo/client.go](./pkg/gloo/client.go)
  - add more tests
- get https:// ref working for plugin download and update kustomize to not use a custom image