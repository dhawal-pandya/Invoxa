package handlers

import (
	sdktracer "github.com/dhawal-pandya/aeonis/packages/tracer-sdk/go"
)

var Tracer *sdktracer.Tracer

func SetTracer(t *sdktracer.Tracer) {
	Tracer = t
}