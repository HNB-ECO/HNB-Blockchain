package bccsp

type ECDSAP256K1KeyGenOpts struct {
	Temporary bool
}

func (opts *ECDSAP256K1KeyGenOpts) Algorithm() string {
	return ECDSAP256
}

func (opts *ECDSAP256K1KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type ECDSAP256KeyGenOpts struct {
	Temporary bool
}

func (opts *ECDSAP256KeyGenOpts) Algorithm() string {
	return ECDSAP256
}

func (opts *ECDSAP256KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type ECDSAP384KeyGenOpts struct {
	Temporary bool
}

func (opts *ECDSAP384KeyGenOpts) Algorithm() string {
	return ECDSAP384
}

func (opts *ECDSAP384KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}
