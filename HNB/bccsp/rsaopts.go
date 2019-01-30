package bccsp

type RSA1024KeyGenOpts struct {
	Temporary bool
}

func (opts *RSA1024KeyGenOpts) Algorithm() string {
	return RSA1024
}

func (opts *RSA1024KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type RSA2048KeyGenOpts struct {
	Temporary bool
}

func (opts *RSA2048KeyGenOpts) Algorithm() string {
	return RSA2048
}

func (opts *RSA2048KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type RSA3072KeyGenOpts struct {
	Temporary bool
}

func (opts *RSA3072KeyGenOpts) Algorithm() string {
	return RSA3072
}

func (opts *RSA3072KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type RSA4096KeyGenOpts struct {
	Temporary bool
}

func (opts *RSA4096KeyGenOpts) Algorithm() string {
	return RSA4096
}

func (opts *RSA4096KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}
