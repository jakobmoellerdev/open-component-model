package internal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	_ "ocm.software/open-component-model/bindings/go/plugin/manager/internal"

	v1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
)

func TestInternallyImportedPlugin(t *testing.T) {
	typ := &v1.OCIImageLayer{
		Type: runtime.Type{
			Version: "OCIImageLayer",
			Name:    "v1",
		},
	}
	p, err := manager.GetReadWriteComponentVersionRepository(context.Background(), nil, typ)
	require.NoError(t, err)
	require.NotNil(t, p)
}
