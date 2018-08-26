
package bccsp

type KeyStore interface {

	ReadOnly() bool

	GetKey(ski []byte) (k Key, err error)

	StoreKey(k Key) (err error)
}
