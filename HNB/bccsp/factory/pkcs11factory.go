package factory

//import (
//	"errors"
//	"fmt"
//
//	"HNB/bccsp"
//	"HNB/bccsp/pkcs11"
//	"HNB/bccsp/sw"
//)
//
//const (
//	PKCS11BasedFactoryName = "PKCS11"
//)
//
//type PKCS11Factory struct{}
//
//func (f *PKCS11Factory) Name() string {
//	return PKCS11BasedFactoryName
//}
//
//func (f *PKCS11Factory) Get(config *FactoryOpts) (bccsp.BCCSP, error) {
//	if config == nil || config.Pkcs11Opts == nil {
//		return nil, errors.New("Invalid config. It must not be nil.")
//	}
//
//	p11Opts := config.Pkcs11Opts
//
//	//TODO: PKCS11 does not need a keystore, but we have not migrated all of PKCS11 BCCSP to PKCS11 yet
//	var ks bccsp.KeyStore
//	if p11Opts.Ephemeral == true {
//		ks = sw.NewDummyKeyStore()
//	} else if p11Opts.FileKeystore != nil {
//		fks, err := sw.NewFileBasedKeyStore(nil, p11Opts.FileKeystore.KeyStorePath, false)
//		if err != nil {
//			return nil, fmt.Errorf("Failed to initialize software key store: %s", err)
//		}
//		ks = fks
//	} else {
//		ks = sw.NewDummyKeyStore()
//	}
//	return pkcs11.New(*p11Opts, ks)
//}
