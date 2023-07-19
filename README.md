**Code:**
[![Go Report Card](https://goreportcard.com/badge/github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform)](https://goreportcard.com/report/github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform)
[![Gateway API plugin CI](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/actions/workflows/ci.yaml/badge.svg)](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/actions/workflows/ci.yaml)

**Social:**
[![Twitter Follow](https://img.shields.io/twitter/follow/argoproj?style=social)](https://twitter.com/argoproj)
[![Slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)

# Argo Rollout Gloo Platform API Plugin

**required rbac in argo-rollouts cluster role**

```yaml
- apiGroups:
  - networking.gloo.solo.io
  resources:
  - routetables
  verbs:
  - '*'
```

**conifgure rollouts configmap with plugin**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
  name: argo-rollouts-config
data:
  trafficRouterPlugins: |-
    - name: "solo-io/glooplatformAPI"
      location: "file://./plugin"
```

**how to reference plugin in a rollout**
```yaml
      trafficRouting:
        plugins:
          solo-io/glooplatformAPI:
            routeTableSelector:
              labels:
                app: demo
              namespace: gloo-mesh
              # name: rt-name
              # namespace: rt-namespace
            routeSelector:
              labels:
                route: demo-preview
              # name: route-name
```