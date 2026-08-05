package omp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type OMPModelRoutingInput struct {
	Catalog        OMPModelCatalog                 `json:"catalog"`
	CatalogReason  string                          `json:"catalog_reason"`
	Routes         map[string]OMPModelRouteRequest `json:"routes"`
	ExecutorFamily string                          `json:"executor_family,omitempty"`
}

type OMPModelRoutingCompilation struct {
	Resolutions      []OMPModelRouteResolution `json:"resolutions"`
	ResolutionDigest string                    `json:"resolution_digest"`
}

type ompRoutingWorkItem struct {
	routeID string
	request OMPModelRouteRequest
}

// CompileOMPModelRouting returns canonical role order independent of map order.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: canonical routing compilation has three production callers.
// @AX:REASON [AUTO]: model integration, doctor projection, and CLI probing require identical ordering and resolution digests.
func CompileOMPModelRouting(input OMPModelRoutingInput) OMPModelRoutingCompilation {
	items := canonicalOMPRoutingWorkItems(input.Routes)
	resolutions := make([]OMPModelRouteResolution, 0, len(items))
	executorFamily := input.ExecutorFamily
	preResolved := make(map[string]OMPModelRouteResolution, 1)

	if executorFamily == "" {
		for _, item := range items {
			if item.request.Agent != "executor" && item.routeID != "executor" {
				continue
			}
			resolved := ResolveOMPModelRoute(input.Catalog, input.CatalogReason, item.request)
			resolved.RouteID = item.routeID
			preResolved[item.routeID] = resolved
			if resolved.Status == "selected" {
				executorFamily = resolved.EffectiveFamily
			}
			break
		}
	}

	for _, item := range items {
		if resolved, ok := preResolved[item.routeID]; ok {
			resolutions = append(resolutions, resolved)
			continue
		}
		request := item.request
		if request.PreferDistinctExecutorFamily && request.ExecutorFamily == "" {
			request.ExecutorFamily = executorFamily
		}
		resolved := ResolveOMPModelRoute(input.Catalog, input.CatalogReason, request)
		resolved.RouteID = item.routeID
		resolutions = append(resolutions, resolved)
	}
	sort.SliceStable(resolutions, func(i, j int) bool {
		left, right := ompRoutingRoleRank(resolutions[i].RequestedRole), ompRoutingRoleRank(resolutions[j].RequestedRole)
		if left != right {
			return left < right
		}
		return resolutions[i].RouteID < resolutions[j].RouteID
	})
	return OMPModelRoutingCompilation{
		Resolutions:      resolutions,
		ResolutionDigest: digestOMPModelRouting(resolutions),
	}
}

func canonicalOMPRoutingWorkItems(routes map[string]OMPModelRouteRequest) []ompRoutingWorkItem {
	items := make([]ompRoutingWorkItem, 0, len(routes))
	for routeID, request := range routes {
		items = append(items, ompRoutingWorkItem{routeID: routeID, request: request})
	}
	sort.Slice(items, func(i, j int) bool {
		left, right := ompRoutingRoleRank(items[i].request.Role), ompRoutingRoleRank(items[j].request.Role)
		if left != right {
			return left < right
		}
		return items[i].routeID < items[j].routeID
	})
	return items
}

func ompRoutingRoleRank(role string) int {
	for index, candidate := range ompRoutingRoleOrder {
		if candidate == role {
			return index
		}
	}
	return len(ompRoutingRoleOrder)
}

func digestOMPModelRouting(resolutions []OMPModelRouteResolution) string {
	payload, err := json.Marshal(struct {
		Resolutions []OMPModelRouteResolution `json:"resolutions"`
	}{Resolutions: resolutions})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}
