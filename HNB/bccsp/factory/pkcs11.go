package factory

//import (
//	"fmt"
//
//	"HNB/bccsp"
//	"HNB/bccsp/pkcs11"
//)
//
//type FactoryOpts struct {
//	ProviderName string             `mapstructure:"default" json:"default" yaml:"Default"`
//	SwOpts       *SwOpts            `mapstructure:"SW,omitempty" json:"SW,omitempty" yaml:"SwOpts"`
//	Pkcs11Opts   *pkcs11.PKCS11Opts `mapstructure:"PKCS11,omitempty" json:"PKCS11,omitempty" yaml:"PKCS11"`
//}
//
//
//func InitFactories(config *FactoryOpts) error {
//	factoriesInitOnce.Do(func() {
//		setFactories(config)
//	})
//
//	return factoriesInitError
//}
//
//func setFactories(config *FactoryOpts) error {
//	if config == nil {
//		config = GetDefaultOpts()
//	}
//
//	if config.ProviderName == "" {
//		config.ProviderName = "SW"
//	}
//
//	if config.SwOpts == nil {
//		config.SwOpts = GetDefaultOpts().SwOpts
//	}
//
//	bccspMap = make(map[string]bccsp.BCCSP)
//
//	if config.SwOpts != nil {
//		f := &SWFactory{}
//		err := initBCCSP(f, config)
//		if err != nil {
//			factoriesInitError = fmt.Errorf("Failed initializing SW.BCCSP [%s]", err)
//		}
//	}
//
//	if config.Pkcs11Opts != nil {
//		f := &PKCS11Factory{}
//		err := initBCCSP(f, config)
//		if err != nil {
//			factoriesInitError = fmt.Errorf("Failed initializing PKCS11.BCCSP %s\n[%s]", factoriesInitError, err)
//		}
//	}
//
//	var ok bool
//	defaultBCCSP, ok = bccspMap[config.ProviderName]
//	if !ok {
//		factoriesInitError = fmt.Errorf("%s\nCould not find default `%s` BCCSP", factoriesInitError, config.ProviderName)
//	}
//
//	return factoriesInitError
//}
//
//func GetBCCSPFromOpts(config *FactoryOpts) (bccsp.BCCSP, error) {
//	var f BCCSPFactory
//	switch config.ProviderName {
//	case "SW":
//		f = &SWFactory{}
//	case "PKCS11":
//		f = &PKCS11Factory{}
//	default:
//		return nil, fmt.Errorf("Could not find BCCSP, no '%s' provider", config.ProviderName)
//	}
//
//	csp, err := f.Get(config)
//	if err != nil {
//		return nil, fmt.Errorf("Could not initialize BCCSP %s [%s]", f.Name(), err)
//	}
//	return csp, nil
//}
