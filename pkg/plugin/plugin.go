package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-glooplatform/pkg/gloo"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/sirupsen/logrus"
	solov2 "github.com/solo-io/solo-apis/client-go/common.gloo.solo.io/v2"
	networkv2 "github.com/solo-io/solo-apis/client-go/networking.gloo.solo.io/v2"
	"k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Type                       = "GlooPlatformAPI"
	GlooPlatformAPIUpdateError = "GlooPlatformAPIUpdateError"
	PluginName                 = "solo-io/glooplatform"
)

type RpcPlugin struct {
	IsTest bool
	// temporary hack until mock clienset is fixed (missing some interface methods)
	// TestRouteTable *networkv2.RouteTable
	LogCtx *logrus.Entry
	Client gloo.NetworkV2ClientSet
}

type GlooPlatformAPITrafficRouting struct {
	RouteTableSelector *DumbObjectSelector `json:"routeTableSelector" protobuf:"bytes,1,name=routeTableSelector"`
	RouteSelector      *DumbRouteSelector  `json:"routeSelector" protobuf:"bytes,2,name=routeSelector"`
	CanarySubsetSelector map[string]string `json:"canarySubsetSelector" protobuf:"bytes,3,name=canarySubsetSelector"`
	StableSubsetSelector map[string]string `json:"stableSubsetSelector" protobuf:"bytes,4,name=stableSubsetSelector"`
}

type DumbObjectSelector struct {
	Labels    map[string]string `json:"labels" protobuf:"bytes,1,name=labels"`
	Name      string            `json:"name" protobuf:"bytes,2,name=name"`
	Namespace string            `json:"namespace" protobuf:"bytes,3,name=namespace"`
}

type DumbRouteSelector struct {
	Labels map[string]string `json:"labels" protobuf:"bytes,1,name=labels"`
	Name   string            `json:"name" protobuf:"bytes,2,name=name"`
}

type GlooDestinationMatcher struct {
	// Regexp *GlooDestinationMatcherRegexp `json:"regexp" protobuf:"bytes,1,name=regexp"`
	Ref *solov2.ObjectReference `json:"ref" protobuf:"bytes,2,name=ref"`
}

type GlooMatchedRouteTable struct {
	// matched gloo platform route table
	RouteTable *networkv2.RouteTable
	// matched http routes within the routetable
	HttpRoutes []*GlooMatchedHttpRoutes
	// matched tcp routes within the routetable
	TCPRoutes []*GlooMatchedTCPRoutes
	// matched tls routes within the routetable
	TLSRoutes []*GlooMatchedTLSRoutes
}

type GlooDestinations struct {
	StableOrActiveDestination  *solov2.DestinationReference
	CanaryOrPreviewDestination *solov2.DestinationReference
}

type GlooMatchedHttpRoutes struct {
	// matched HttpRoute
	HttpRoute *networkv2.HTTPRoute
	// matched destinations within the httpRoute
	Destinations *GlooDestinations
}

type GlooMatchedTLSRoutes struct {
	// matched HttpRoute
	TLSRoute *networkv2.TLSRoute
	// matched destinations within the httpRoute
	Destinations []*GlooDestinations
}

type GlooMatchedTCPRoutes struct {
	// matched HttpRoute
	TCPRoute *networkv2.TCPRoute
	// matched destinations within the httpRoute
	Destinations []*GlooDestinations
}

func (r *RpcPlugin) InitPlugin() pluginTypes.RpcError {
	if r.IsTest {
		return pluginTypes.RpcError{}
	}
	client, err := gloo.NewNetworkV2ClientSet()
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}
	r.Client = client
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	ctx := context.TODO()
	//todo - validation on unmarshal config
 	glooPluginConfig, err := getPluginConfig(rollout)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	// get the matched routetables
	matchedRts, err := r.getRouteTables(ctx, rollout, glooPluginConfig)
	if err != nil {
		return pluginTypes.RpcError{
			ErrorString: err.Error(),
		}
	}

	if rollout.Spec.Strategy.Canary != nil {
		return r.handleCanary(ctx, rollout, desiredWeight, additionalDestinations, glooPluginConfig, matchedRts)
	} else if rollout.Spec.Strategy.BlueGreen != nil {
		return r.handleBlueGreen(rollout)
	}

	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	// we could remove the canary destination, but not required since it will have 0 weight at the end of rollout
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}

func getPluginConfig(rollout *v1alpha1.Rollout) (*GlooPlatformAPITrafficRouting, error) {
	glooplatformConfig := GlooPlatformAPITrafficRouting{}

	err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[PluginName], &glooplatformConfig)
	if err != nil {
		return nil, err
	}

	return &glooplatformConfig, nil
}

// getRouteTables matches the routeTables based on the route table selectors in the plugin config from the Rollout CR.
// If a RouteTableSelector is provided, it will match the RouteTable based on the provided namespace and name.
// If a namespace is not provided, it will default to the Rollout's namespace.
// If a name is not provided, it will match based on labels in the provided namespace.
// Label matchers can match multiple route tables, and the plugin will attempt to match the routes within each route table.
func (r *RpcPlugin) getRouteTables(ctx context.Context, rollout *v1alpha1.Rollout, glooPluginConfig *GlooPlatformAPITrafficRouting) ([]*GlooMatchedRouteTable, error) {
	if glooPluginConfig.RouteTableSelector == nil {
		return nil, fmt.Errorf("routeTable selector is required")
	}

	if strings.EqualFold(glooPluginConfig.RouteTableSelector.Namespace, "") {
		r.LogCtx.Debugf("defaulting routeTableSelector namespace to Rollout namespace %s for rollout %s", rollout.Namespace, rollout.Name)
		glooPluginConfig.RouteTableSelector.Namespace = rollout.Namespace
	}

	var rts []*networkv2.RouteTable

	if !strings.EqualFold(glooPluginConfig.RouteTableSelector.Name, "") {
		r.LogCtx.Debugf("getRouteTables using ns:name ref %s:%s to get single table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		result, err := r.Client.RouteTables().GetRouteTable(ctx, glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		if err != nil {
			return nil, err
		}

		r.LogCtx.Debugf("getRouteTables using ns:name ref %s:%s found 1 table", glooPluginConfig.RouteTableSelector.Name, glooPluginConfig.RouteTableSelector.Namespace)
		rts = append(rts, result)
	} else {
		opts := &k8sclient.ListOptions{}

		if glooPluginConfig.RouteTableSelector.Labels != nil {
			opts.LabelSelector = labels.SelectorFromSet(glooPluginConfig.RouteTableSelector.Labels)
		}
		if !strings.EqualFold(glooPluginConfig.RouteTableSelector.Namespace, "") {
			opts.Namespace = glooPluginConfig.RouteTableSelector.Namespace
		}

		r.LogCtx.Debugf("getRouteTables listing tables with opts %+v", opts)
		var err error

		rts, err = r.Client.RouteTables().ListRouteTable(ctx, opts)
		if err != nil {
			return nil, err
		}
		r.LogCtx.Debugf("getRouteTables listing tables with opts %+v; found %d routeTables", opts, len(rts))
	}

	matched := []*GlooMatchedRouteTable{}

	for _, rt := range rts {
		matchedRt := &GlooMatchedRouteTable{
			RouteTable: rt,
		}
		// destination matching
		if err := matchedRt.matchRoutes(r.LogCtx, rollout, glooPluginConfig); err != nil {
			return nil, err
		}

		matched = append(matched, matchedRt)
	}

	return matched, nil
}

// matchRoutes matches the routes within the RouteTable based on the provided route selectors in the plugin config from the Rollout CR.
// 
func (g *GlooMatchedRouteTable) matchRoutes(logCtx *logrus.Entry, rollout *v1alpha1.Rollout, trafficConfig *GlooPlatformAPITrafficRouting) error {
	if g.RouteTable == nil {
		return fmt.Errorf("matchRoutes called for nil RouteTable")
	}

	// HTTP Routes
	for _, httpRoute := range g.RouteTable.Spec.Http {
		// find the destination that matches the stable svc
		fw := httpRoute.GetForwardTo()
		if fw == nil {
			logCtx.Debugf("skipping route %s.%s because forwardTo is nil", g.RouteTable.Name, httpRoute.Name)
			continue
		}

		// skip non-matching routes if RouteSelector provided
		if trafficConfig.RouteSelector != nil {
			// if name was provided, skip if route name doesn't match
			if !strings.EqualFold(trafficConfig.RouteSelector.Name, "") && !strings.EqualFold(trafficConfig.RouteSelector.Name, httpRoute.Name) {
				logCtx.Debugf("skipping route %s.%s because it doesn't match route name selector %s", g.RouteTable.Name, httpRoute.Name, trafficConfig.RouteSelector.Name)
				continue
			}
			// if labels provided, skip if route labels do not contain all specified labels
			if trafficConfig.RouteSelector.Labels != nil {
				matchedLabels := func() bool {
					for k, v := range trafficConfig.RouteSelector.Labels {
						if vv, ok := httpRoute.Labels[k]; ok {
							if !strings.EqualFold(v, vv) {
								logCtx.Debugf("skipping route %s.%s because route labels do not contain %s=%s", g.RouteTable.Name, httpRoute.Name, k, v)
								return false
							}
						}
					}
					return true
				}()
				if !matchedLabels {
					continue
				}
			}
			logCtx.Debugf("route %s.%s passed RouteSelector", g.RouteTable.Name, httpRoute.Name)
		}

		// find destinations
		// var matchedDestinations []*GlooDestinations
		var canary, stable *solov2.DestinationReference
		for _, dest := range fw.Destinations {
			ref := dest.GetRef()
			if ref == nil {
				logCtx.Debugf("skipping destination %s.%s because destination ref was nil; %+v", g.RouteTable.Name, httpRoute.Name, dest)
				continue
			}
			if strings.EqualFold(ref.Name, rollout.Spec.Strategy.Canary.StableService) {
				logCtx.Debugf("matched stable ref %s.%s.%s", g.RouteTable.Name, httpRoute.Name, ref.Name)
				stable = dest
				continue
			}
			if strings.EqualFold(ref.Name, rollout.Spec.Strategy.Canary.CanaryService) {
				logCtx.Debugf("matched canary ref %s.%s.%s", g.RouteTable.Name, httpRoute.Name, ref.Name)
				canary = dest
				// bail if we found both stable and canary
				if stable != nil {
					break
				}
				continue
			}
		}

		if canary != nil {
			if !mapsEqual(canary.Subset, trafficConfig.CanarySubsetSelector) {
				logCtx.Debugf("skipping route %s.%s because canary subset does not match specified canary subset selector ", g.RouteTable.Name, httpRoute.Name)
				continue
			}
		}

		if stable != nil {
			if !mapsEqual(stable.Subset, trafficConfig.StableSubsetSelector) {
				logCtx.Debugf("skipping route %s.%s because stable subset does not match specified stable subset selector", g.RouteTable.Name, httpRoute.Name)
				continue
			}
			dest := &GlooMatchedHttpRoutes{
				HttpRoute: httpRoute,
				Destinations: &GlooDestinations{
					StableOrActiveDestination:  stable,
					// Canary can be nil here, we will create it later
					CanaryOrPreviewDestination: canary,
				},
			}
			logCtx.Debugf("adding destination %+v", dest)
			g.HttpRoutes = append(g.HttpRoutes, dest)
		}
	} // end range httpRoutes

	return nil
}

func mapsEqual(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
			return false
	}
	for k, v1 := range m1 {
			if v2, ok := m2[k]; !ok || v1 != v2 {
					return false
			}
	}
	return true
}