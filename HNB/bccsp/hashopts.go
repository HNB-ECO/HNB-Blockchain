package bccsp

import "fmt"

type SHA256Opts struct {
}

func (opts *SHA256Opts) Algorithm() string {
	return SHA256
}

type SHA384Opts struct {
}

func (opts *SHA384Opts) Algorithm() string {
	return SHA384
}

type SHA3_256Opts struct {
}

func (opts *SHA3_256Opts) Algorithm() string {
	return SHA3_256
}

type SHA3_384Opts struct {
}

func (opts *SHA3_384Opts) Algorithm() string {
	return SHA3_384
}

func GetHashOpt(hashFunction string) (HashOpts, error) {
	switch hashFunction {
	case SHA256:
		return &SHA256Opts{}, nil
	case SHA384:
		return &SHA384Opts{}, nil
	case SHA3_256:
		return &SHA3_256Opts{}, nil
	case SHA3_384:
		return &SHA3_384Opts{}, nil
	}
	return nil, fmt.Errorf("hash function not recognized [%s]", hashFunction)
}
