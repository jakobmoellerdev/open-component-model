// Package internal contains a test plugin that is registered using internal
// functions of as a transfer plugin.
package internal

import (
	"context"
	"fmt"
	"os"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	oci "ocm.software/open-component-model/bindings/go/oci/spec/access"
	v1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
)

type MyInternalPlugin[T runtime.Typed] struct{}

func (m *MyInternalPlugin[T]) Ping(ctx context.Context) error {
	return nil
}

func (m *MyInternalPlugin[T]) GetComponentVersion(ctx context.Context, request manager.GetComponentVersionRequest[T], credentials manager.Attributes) (*descriptor.Descriptor, error) {
	_, _ = fmt.Fprintf(os.Stdout, "GetComponentVersion[%s %s]\n", request.Name, request.Version)

	return nil, nil
}

func (m *MyInternalPlugin[T]) GetLocalResource(ctx context.Context, request manager.GetLocalResourceRequest[T], credentials manager.Attributes) error {
	_, _ = fmt.Fprintf(os.Stdout, "GetLocalResource[%s %s]\n", request.Name, request.Version)

	return nil
}

func (m *MyInternalPlugin[T]) AddLocalResource(ctx context.Context, request manager.PostLocalResourceRequest[T], credentials manager.Attributes) (*descriptor.Resource, error) {
	_, _ = fmt.Fprintf(os.Stdout, "AddLocalResource[%s %s]\n", request.Name, request.Version)

	return nil, nil
}

func (m *MyInternalPlugin[T]) AddComponentVersion(ctx context.Context, request manager.PostComponentVersionRequest[T], credentials manager.Attributes) error {
	_, _ = fmt.Fprintf(os.Stdout, "AddComponentVersion[%s %s]\n", request.Descriptor.Component.Name, request.Descriptor.Component.Version)

	return nil
}

var _ manager.ReadWriteOCMRepositoryPluginContract[runtime.Typed] = &MyInternalPlugin[runtime.Typed]{}

func init() {
	scheme := runtime.NewScheme()
	oci.MustAddToScheme(scheme)
	if err := manager.RegisterInternalComponentVersionRepositoryPlugin(scheme, &MyInternalPlugin[*v1.OCIImageLayer]{}, &v1.OCIImageLayer{}); err != nil {
		panic(err)
	}
}
