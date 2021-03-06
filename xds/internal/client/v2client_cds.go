/*
 *
 * Copyright 2019 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package client

import (
	"fmt"

	xdspb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/golang/protobuf/ptypes"
)

// handleCDSResponse processes an CDS response received from the xDS server. On
// receipt of a good response, it also invokes the registered watcher callback.
func (v2c *v2Client) handleCDSResponse(resp *xdspb.DiscoveryResponse) error {
	returnUpdate := make(map[string]ClusterUpdate)
	for _, r := range resp.GetResources() {
		var resource ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(r, &resource); err != nil {
			return fmt.Errorf("xds: failed to unmarshal resource in CDS response: %v", err)
		}
		cluster, ok := resource.Message.(*xdspb.Cluster)
		if !ok {
			return fmt.Errorf("xds: unexpected resource type: %T in CDS response", resource.Message)
		}
		v2c.logger.Infof("Resource with name: %v, type: %T, contains: %v", cluster.GetName(), cluster, cluster)
		update, err := validateCluster(cluster)
		if err != nil {
			return err
		}

		// If the Cluster message in the CDS response did not contain a
		// serviceName, we will just use the clusterName for EDS.
		if update.ServiceName == "" {
			update.ServiceName = cluster.GetName()
		}
		v2c.logger.Debugf("Resource with name %v, type %T, value %+v added to cache", cluster.GetName(), update, update)
		returnUpdate[cluster.GetName()] = update
	}

	v2c.parent.newCDSUpdate(returnUpdate)
	return nil
}

func validateCluster(cluster *xdspb.Cluster) (ClusterUpdate, error) {
	emptyUpdate := ClusterUpdate{ServiceName: "", EnableLRS: false}
	switch {
	case cluster.GetType() != xdspb.Cluster_EDS:
		return emptyUpdate, fmt.Errorf("xds: unexpected cluster type %v in response: %+v", cluster.GetType(), cluster)
	case cluster.GetEdsClusterConfig().GetEdsConfig().GetAds() == nil:
		return emptyUpdate, fmt.Errorf("xds: unexpected edsConfig in response: %+v", cluster)
	case cluster.GetLbPolicy() != xdspb.Cluster_ROUND_ROBIN:
		return emptyUpdate, fmt.Errorf("xds: unexpected lbPolicy %v in response: %+v", cluster.GetLbPolicy(), cluster)
	}

	return ClusterUpdate{
		ServiceName: cluster.GetEdsClusterConfig().GetServiceName(),
		EnableLRS:   cluster.GetLrsServer().GetSelf() != nil,
	}, nil
}
