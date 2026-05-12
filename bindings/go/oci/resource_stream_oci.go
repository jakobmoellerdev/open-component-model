package oci

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"

	"ocm.software/open-component-model/bindings/go/oci/stream"
)

func newResourceStream(store content.ReadOnlyStorage, root ocispec.Descriptor, copyOpts oras.CopyGraphOptions, tempDir string, tags []string) *stream.OCIResourceStream {
	return stream.New(store, root, copyOpts, tempDir, tags)
}
