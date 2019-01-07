package factory

func GetDefaultOpts() *FactoryOpts {
	return &FactoryOpts{
		ProviderName: "SW",
		SwOpts: &SwOpts{
			HashFamily: "SHA2",
			SecLevel:   256,

			Ephemeral: true,
		},
	}
}

func (o *FactoryOpts) FactoryName() string {
	return o.ProviderName
}
