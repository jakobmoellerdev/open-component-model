package oci

import (
	"context"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"

	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/oci/tar"
)

// ociResourceStream wraps a content.ReadOnlyStorage (typically a remote.Repository)
// and a resolved root descriptor. No network I/O occurs at construction time.
type ociResourceStream struct {
	store    content.ReadOnlyStorage
	root     ocispec.Descriptor
	copyOpts oras.CopyGraphOptions
	tempDir  string
	tags     []string
}

var _ ResourceStream = (*ociResourceStream)(nil)

func newResourceStream(store content.ReadOnlyStorage, root ocispec.Descriptor, copyOpts oras.CopyGraphOptions, tempDir string, tags []string) *ociResourceStream {
	return &ociResourceStream{
		store:    store,
		root:     root,
		copyOpts: copyOpts,
		tempDir:  tempDir,
		tags:     tags,
	}
}

func (s *ociResourceStream) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	return s.store.Fetch(ctx, desc)
}

func (s *ociResourceStream) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	return s.store.Exists(ctx, desc)
}

func (s *ociResourceStream) Root() ocispec.Descriptor {
	return s.root
}

func (s *ociResourceStream) Materialize(ctx context.Context) (blob.ReadOnlyBlob, error) {
	return tar.CopyToOCILayoutInMemory(ctx, s.store, s.root, tar.CopyToOCILayoutOptions{
		CopyGraphOptions: s.copyOpts,
		Tags:             s.tags,
		TempDir:          s.tempDir,
	})
}
