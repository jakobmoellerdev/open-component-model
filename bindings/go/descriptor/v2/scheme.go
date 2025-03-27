package v2

import (
	"github.com/jakobmoellerdev/open-component-model/bindings/go/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	MustAddToScheme(Scheme)
}

func MustAddToScheme(scheme *runtime.Scheme) {
	obj := &LocalBlob{}
	scheme.MustRegisterWithAlias(obj, runtime.NewType(LocalBlobAccessTypeGroup, LocalBlobAccessType, LocalBlobAccessTypeVersion))
}
