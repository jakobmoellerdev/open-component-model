package status

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatusPatcher tracks an object's state to compute and apply merge patches
// for both the object itself and its status sub-resource.
type StatusPatcher struct {
	client       client.Client
	beforeObject client.Object
}

// NewStatusPatcher returns a StatusPatcher with the given object as the initial
// base object for the patching operations.
func NewStatusPatcher(obj client.Object, c client.Client) *StatusPatcher {
	return &StatusPatcher{
		client:       c,
		beforeObject: obj.DeepCopyObject().(client.Object),
	}
}

// Patch computes a merge patch from the baseline snapshot and applies it to
// both the status sub-resource and the object itself. After a successful patch,
// the baseline is updated for subsequent calls.
func (sp *StatusPatcher) Patch(ctx context.Context, obj client.Object) error {
	if err := sp.client.Status().Patch(ctx, obj, client.MergeFrom(sp.beforeObject)); err != nil {
		return err
	}
	if err := sp.client.Patch(ctx, obj, client.MergeFrom(sp.beforeObject)); err != nil {
		return err
	}
	sp.beforeObject = obj.DeepCopyObject().(client.Object)
	return nil
}
