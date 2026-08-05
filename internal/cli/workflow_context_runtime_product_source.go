package cli

import (
	"context"
	"fmt"
	"reflect"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type workflowContextProductCanonicalSource struct {
	options   promptlayer.ContextDeliveryOptions
	ephemeral promptlayer.OMPContextEphemeral
}

func (source workflowContextProductCanonicalSource) Rebuild(
	_ context.Context,
	options promptlayer.ContextDeliveryOptions,
) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
	if !reflect.DeepEqual(source.options, options) {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{},
			fmt.Errorf("authoritative OMP context options changed")
	}
	delivery, err := promptlayer.BuildContextDelivery(source.options)
	if err != nil {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{}, err
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(source.options, delivery); err != nil {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{}, err
	}
	ephemeral := source.ephemeral
	ephemeral.FrozenFindingIDs = append([]string(nil), source.ephemeral.FrozenFindingIDs...)
	ephemeral.OwnershipPaths = append([]string(nil), source.ephemeral.OwnershipPaths...)
	ephemeral.ForbiddenPaths = append([]string(nil), source.ephemeral.ForbiddenPaths...)
	return delivery, ephemeral, nil
}
