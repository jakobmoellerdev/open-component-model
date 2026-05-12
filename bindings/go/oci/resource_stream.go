package oci

import "ocm.software/open-component-model/bindings/go/oci/stream"

// ResourceStream is a lazy handle to OCI content.
// See the stream package for full documentation.
type ResourceStream = stream.ResourceStream

// StreamingResourceRepository extends the generic ResourceRepository with
// OCI-native streaming. See the stream package for full documentation.
type StreamingResourceRepository = stream.Repository
