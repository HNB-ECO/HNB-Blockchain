package factory

import (
	"fmt"
	"sync"

	"HNB/bccsp"
	"HNB/logging"
)

const LOGTABLE_BCCSP string = "bccsp"

var (
	defaultBCCSP bccsp.BCCSP

	bootBCCSP bccsp.BCCSP

	bccspMap map[string]bccsp.BCCSP

	factoriesInitOnce sync.Once
	bootBCCSPInitOnce sync.Once

	factoriesInitError error

	logger = logging.GetLogIns()
)

type BCCSPFactory interface {
	Name() string

	Get(opts *FactoryOpts) (bccsp.BCCSP, error)
}

func GetDefault() bccsp.BCCSP {
	if defaultBCCSP == nil {
		//logger.Warning(LOGTABLE_BCCSP,"Before using BCCSP, please call InitFactories(). Falling back to bootBCCSP.")
		bootBCCSPInitOnce.Do(func() {
			var err error
			f := &SWFactory{}
			bootBCCSP, err = f.Get(GetDefaultOpts())
			if err != nil {
				panic("BCCSP Internal error, failed initialization with GetDefaultOpts!")
			}
		})
		return bootBCCSP
	}
	return defaultBCCSP
}

func GetBCCSP(name string) (bccsp.BCCSP, error) {
	csp, ok := bccspMap[name]
	if !ok {
		return nil, fmt.Errorf("Could not find BCCSP, no '%s' provider", name)
	}
	return csp, nil
}

func initBCCSP(f BCCSPFactory, config *FactoryOpts) error {
	csp, err := f.Get(config)
	if err != nil {
		return fmt.Errorf("Could not initialize BCCSP %s [%s]", f.Name(), err)
	}

	logger.Debugf(LOGTABLE_BCCSP, "Initialize BCCSP [%s]", f.Name())
	bccspMap[f.Name()] = csp
	return nil
}
