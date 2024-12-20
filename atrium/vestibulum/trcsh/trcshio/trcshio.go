package trcshio

import "io"

type TrcshReadWriteCloser interface {
	io.ReadWriteCloser
	Name() string
}
