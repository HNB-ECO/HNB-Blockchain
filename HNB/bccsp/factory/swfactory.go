package factory

import (
	"errors"
	"fmt"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/sw"
)

const (
	SoftwareBasedFactoryName = "SW"
)

type SWFactory struct{}

func (f *SWFactory) Name() string {
	return SoftwareBasedFactoryName
}

func (f *SWFactory) Get(config *FactoryOpts) (bccsp.BCCSP, error) {
	if config == nil || config.SwOpts == nil {
		return nil, errors.New("Invalid config. It must not be nil.")
	}

	swOpts := config.SwOpts

	var ks bccsp.KeyStore
	if swOpts.Ephemeral == true {
		ks = sw.NewDummyKeyStore()
	} else if swOpts.FileKeystore != nil {
		fks, err := sw.NewFileBasedKeyStore(nil, swOpts.FileKeystore.KeyStorePath, false)
		if err != nil {
			return nil, fmt.Errorf("Failed to initialize software key store: %s", err)
		}
		ks = fks
	} else {
		ks = sw.NewDummyKeyStore()
	}

	return sw.New(swOpts.SecLevel, swOpts.HashFamily, ks)
}

type SwOpts struct {
	SecLevel   int    `mapstructure:"security" json:"security" yaml:"Security"`
	HashFamily string `mapstructure:"hash" json:"hash" yaml:"Hash"`

	Ephemeral     bool               `mapstructure:"tempkeys,omitempty" json:"tempkeys,omitempty"`
	FileKeystore  *FileKeystoreOpts  `mapstructure:"filekeystore,omitempty" json:"filekeystore,omitempty" yaml:"FileKeyStore"`
	DummyKeystore *DummyKeystoreOpts `mapstructure:"dummykeystore,omitempty" json:"dummykeystore,omitempty"`
}

type FileKeystoreOpts struct {
	KeyStorePath string `mapstructure:"keystore" yaml:"KeyStore"`
}

type DummyKeystoreOpts struct{}
