module gnark_on_icicle

go 1.21.6

require (
	github.com/consensys/gnark v0.9.1
	github.com/consensys/gnark-crypto v0.12.2-0.20231208203441-d4eab6ddd2af
)

require (
	github.com/NVIDIA/go-nvml v0.12.0-2
	github.com/bits-and-blooms/bitset v1.13.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.6.0 // indirect
	github.com/google/pprof v0.0.0-20240227163752-401108e1b7e7 // indirect
	github.com/ingonyama-zk/icicle v1.0.0 // indirect
	github.com/ingonyama-zk/iciclegnark v0.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.32.0
	github.com/stretchr/testify v1.8.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.20.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)

replace github.com/consensys/gnark => github.com/celer-network/gnark v0.0.0-20240103092544-e37f964b1a96

replace github.com/ingonyama-zk/icicle v1.0.0 => github.com/ingonyama-zk/icicle v0.1.0
