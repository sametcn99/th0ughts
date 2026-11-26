//go:build !nativekv || !cgo

package storage

import "errors"

const NativeKVEnabled = false

func openNative(Config) (Store, error) {
	return nil, errors.New("this build omits native storage; use a release binary or build with the nativekv tag and cgo")
}
