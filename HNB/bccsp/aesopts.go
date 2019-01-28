package bccsp

type AES128KeyGenOpts struct {
	Temporary bool
}

func (opts *AES128KeyGenOpts) Algorithm() string {
	return AES128
}

func (opts *AES128KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type AES192KeyGenOpts struct {
	Temporary bool
}

func (opts *AES192KeyGenOpts) Algorithm() string {
	return AES192
}

func (opts *AES192KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}

type AES256KeyGenOpts struct {
	Temporary bool
}

func (opts *AES256KeyGenOpts) Algorithm() string {
	return AES256
}

func (opts *AES256KeyGenOpts) Ephemeral() bool {
	return opts.Temporary
}
