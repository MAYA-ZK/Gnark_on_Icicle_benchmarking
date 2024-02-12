# Gnark_on_Icicle_benchmarking
Repo for Benchmarking the Gnark ZK Library using Icicle for GPU accelration
# Getting Started
## Requirements
- Ubuntu installation (tested on version 22.04.3 LTS)
- Docker
- Nvidia GPU
- Nvidia GPU drivers
## Setting up the docker container
To ease reproduceability, this repo is designed to run on a docker container. The docker container is defined in the provided `Docekrfile` and is based on a Ubuntu 20.04 with CUDA Toolkit 12.2 pre-installed.

After cloning the repo on your machine, build the container by running:

`docker build -t gnark-benchmark:dev .`

Then, start the container:

`nvidia-docker run -it -d --runtime nvidia --ipc=host -v $(pwd):/home --name gnark-benchmark gnark-benchmark:dev`

## Instal the necessary dependencies

Open a terminal inside the container (by using the VS Code extension Dev containers for example) or by simply running the following command:

`nvidia-docker exec -it gnark_benchmark /bin/bash`

Before continuing we need to install `nano` and `wget` to be able to edit files from the terminal and download files

`apt update && apt install wget nano`

Download and install Go:
```
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
rm go1.21.6.linux-amd64.tar.gz
```
Add the Go installation directory to PATH by adding the following line to `/etc/.profile`

`export PATH=$PATH:/usr/local/go/bin`

Update the PATH variable

`source /etc/.profile`

Check if Go was properly installed

`go version`

## Installing the go modules
Go back to the home directory (where the repo is cloned) and install the required Go modules

`go get -u`

## Building the Icicle shared libraries

Before running the code we need to create the icicle share libraries for each curve.

To do that go to `/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle` and run:

`make all -j 4`

After the libraries are built, their PATH needs to be exported. Note that you need to run these two line everytime you restart the container or open a new terminal

```
export CGO_LD_FLAGS=-L/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle/
```
## Circuits
For benchmarking we use 3 different circuits: cubic, exponentiate and sha256. 
They are defined as follows:
- cubic:
  -  Inputs: `y` : arbitrary size integer / public, `x` arbitrary size integer / secret, 
  -  Constraint: `y = x^3 + x + 5`
- exponentiate: 
  - Inputs: `x`: arbitrary size unsigned integer / public, `y`: arbitrary size unsigned integer / public, `e`: 8-bit unsigned inetger / secret
  - Constraint: `y == x^e`
- sha256:
  - Inputs: `hash`: 32 byte hexadecimal string / public, 'preimage': arbitrary size hexadecimal string / secret
  - Constraint: `sha256(preimage) == hash

### Note regarding input sizes
While a lot of the inputs are defined ith arbitrary size, when generating random inputs the inputs are actually capped in size. The size of the randomly generated inputs are cdeined using constants in the package defining each sircuit respectively. These of course can be changed to generate smaller or larger inputs. Here's a list of these constants:
- `X_SIZE` in `cubic/cubic.go`: sets the size of the randomly generated x in the cubic circuit in bytes. Default value is 8
- `X_SIZE` in `exponentiate/exponentiate.go`: sets the size of the randomly generated x in the exponentiate circuit in bytes. Default value is 16
- `E_BITSIZE` in `exponentiate/exponentiate.go`: sets the size of the randomly generated e in the cubic circuit in bits. Default value is 8. DO NOT CHANGE THIS VALUE!
- `PREIMAGE_SIZE` in `sha256/sha256.go`: sets the size of the randomly generated pre-image in the sha256 circuit in bytes. Default value is 32. Note taht you also need to change this constant when using a file as input when benchmarking the sha256 circuit to reflect the pre-images' size in the file.

## Running Benchmarks
To run the benchmarks simply run the `main.go`. Here's an example command:
`go run -tags=icicle main.go -curve bn254 -GPU_Acc -circuit sha256 -n 10 `

The programs takes inputs for benchmarking in two methods:
- The first method is by generating n random inputs and calculating the outputs. Note that n has a max value set by the constant `MAX_INPUTS` under `main.go`
- The second method is by having inputs defined in a file where each line represents a set of inputs seperated by spaces. The order of inputs in the files for the 3 circuits is as follow:
  - cubic: x, y
  - exponentiate: x, y, e
  - sha256: hash, pre-image (make sure both are written in hexadecimal and not decimal)
Examples for files for each circuit are founder under `./inputs/`
The program takes the following arguments:

| Argument         | Description                            | Type         | Possible Values                      | Default Value |
|------------------|----------------------------------------|--------------|--------------------------------------|---------------|
| `-curve`         | Specify the curve for the ZK-Snark     | string       | bn254, bls12_377, bls12_381, bw6_761 | bn254         |
| `-GPU_Acc`       | Enable/disable GPU acceleration        | bool         | true, flase                          | false         |
| `-circuit`       | Choose the circuit to becnhmark        | string       | cubic, exponentiate, sha256          | sha256        |
| `-file_path`     | Path to file with pre_computed inputs  | string       | empty string                         | empty string  |
| `-n`             | Number of inputs to generate randomly  | int          | all integer values                   | 10            |


