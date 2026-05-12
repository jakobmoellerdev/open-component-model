package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func TestOCIResourceStream_Fetch(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	layerContent := []byte("hello layer")
	layerDesc := pushBlob(t, ctx, store, ocispec.MediaTypeImageLayer, layerContent)

	stream := newResourceStream(store, layerDesc, oras.DefaultCopyGraphOptions, "", nil)

	rc, err := stream.Fetch(ctx, layerDesc)
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, layerContent, got)
}

func TestOCIResourceStream_Exists(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	layerContent := []byte("exists check")
	layerDesc := pushBlob(t, ctx, store, ocispec.MediaTypeImageLayer, layerContent)

	stream := newResourceStream(store, layerDesc, oras.DefaultCopyGraphOptions, "", nil)

	exists, err := stream.Exists(ctx, layerDesc)
	require.NoError(t, err)
	assert.True(t, exists)

	missingDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    digest.FromString("missing"),
		Size:      7,
	}
	exists, err = stream.Exists(ctx, missingDesc)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestOCIResourceStream_Root(t *testing.T) {
	store := memory.New()
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromString("test"),
		Size:      4,
	}

	stream := newResourceStream(store, desc, oras.DefaultCopyGraphOptions, "", nil)
	assert.Equal(t, desc, stream.Root())
}

func TestOCIResourceStream_Materialize(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Push a config blob
	configContent := []byte("{}")
	configDesc := pushBlob(t, ctx, store, ocispec.MediaTypeImageConfig, configContent)

	// Push a layer blob
	layerContent := []byte("layer data for materialize test")
	layerDesc := pushBlob(t, ctx, store, ocispec.MediaTypeImageLayer, layerContent)

	// Push a manifest
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestDesc := pushBlob(t, ctx, store, ocispec.MediaTypeImageManifest, manifestBytes)

	stream := newResourceStream(store, manifestDesc, oras.DefaultCopyGraphOptions, t.TempDir(), nil)

	blob, err := stream.Materialize(ctx)
	require.NoError(t, err)

	rc, err := blob.ReadCloser()
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.True(t, len(data) > 0, "materialized blob should have content")
}

func TestOCIResourceStream_CopyGraph(t *testing.T) {
	ctx := context.Background()
	srcStore := memory.New()
	dstStore := memory.New()

	// Push a config blob
	configContent := []byte("{}")
	configDesc := pushBlob(t, ctx, srcStore, ocispec.MediaTypeImageConfig, configContent)

	// Push a layer blob
	layerContent := []byte("streaming layer data")
	layerDesc := pushBlob(t, ctx, srcStore, ocispec.MediaTypeImageLayer, layerContent)

	// Push a manifest
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    configDesc,
		Layers:    []ocispec.Descriptor{layerDesc},
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestDesc := pushBlob(t, ctx, srcStore, ocispec.MediaTypeImageManifest, manifestBytes)

	// Create stream from source and copy to destination
	stream := newResourceStream(srcStore, manifestDesc, oras.DefaultCopyGraphOptions, "", nil)

	err = oras.CopyGraph(ctx, stream, dstStore, stream.Root(), oras.DefaultCopyGraphOptions)
	require.NoError(t, err)

	// Verify all blobs exist in destination
	exists, err := dstStore.Exists(ctx, manifestDesc)
	require.NoError(t, err)
	assert.True(t, exists, "manifest should exist in destination")

	exists, err = dstStore.Exists(ctx, configDesc)
	require.NoError(t, err)
	assert.True(t, exists, "config should exist in destination")

	exists, err = dstStore.Exists(ctx, layerDesc)
	require.NoError(t, err)
	assert.True(t, exists, "layer should exist in destination")

	// Verify layer content matches
	rc, err := dstStore.Fetch(ctx, layerDesc)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, layerContent, got)
}

func pushBlob(t *testing.T, ctx context.Context, store *memory.Store, mediaType string, content []byte) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	err := store.Push(ctx, desc, bytes.NewReader(content))
	require.NoError(t, err)
	return desc
}
