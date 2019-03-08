## Requirements

[Go](http://golang.org/) 1.11 or newer.

## Installation
#### Linux/BSD/MacOSX/POSIX - Build from Source

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path.  It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues.  It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain hnb, all dependencies, and install it:

```bash
$ go get -u github.com/Masterminds/glide
$ git clone https://github.com/HNB-ECO/HNB-Blockchain.git $GOPATH/src/github.com/HNB-ECO/HNB-Blockchain/
$ cd $GOPATH/src/github.com/HNB-ECO/HNB-Blockchain/HNB
$ glide install
$ cd start
$ go build start.go
```
## Getting Started

hnb has several configuration options available to tweak how it runs, but all
of the basic operations described in the intro section work with zero
configuration.

#### Linux/BSD/POSIX/Source

```bash
$ ./start
```


|Directory|Brief Description|
|:--------|:----------------|
|access/rest|The interface to access this service friendly, includes RESTful, gRPC etc.|
|account|The management of the node account with node address and node keys.|
|appMgr|The framework of HNB, HGS transaction management. This module can also be the framework of scale-out to support smart contracts life circle management. It also provides the run-time environment of smart contracts.|
|bccsp|Blockchain crypto service provider with rsa, ecdsa, aes algorithms. Implementation of pkcs11 interface. And sw is the implementation of crypto without hard equipment.|
|blockStore|Defines the struct of block and implements of wirte and read blocks.
|cli|The command line interface for developers to easy to test or debug.
|common|The definition of common data struct of this project, such as: merkel tree and transaction.
|config|The configeration management which is relied by start package.
|consensus|Consensus algorithm, the implementation of VRF(Verifiable Random Function) , Algorand and DPoS.
|contract/hgs|The package of native data assets business management and other Dapp code management.
|db|The management of  data persistence operations with the implementation of K-V database, Newsql database, and file storage as well.
|ledger|The ledger management of with the read and write operations of chain blocks and smart contract state data.
|logging|The logger module.
|msgBus|The message transmitte service of subscription and publish messages between different modules.
|p2pNetwork|The peer to peer network management to help the node to manage the neighbor nodes to build the whole peer to peer network.
|start|The entrance of this project. Starts and initializes whole system.
|txpool|The transaction pool manager which manages transactions from the phase of transaction received to the phase of consensus.
|util|Utilities of HNB project, such as: timer util+C, date and time util, file util.

