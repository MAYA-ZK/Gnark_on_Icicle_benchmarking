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

`docker run -it -d --runtime nvidia --ipc=host -v $(pwd):/home --name gnark-benchmark gnark-benchmark:dev`

## Instal the necessary dependencies

Open a terminal inside the container (by using the VS Code extension Dev containers for example) or by simply running the following command:

`docekr exec -it gnark_benchmark /bin/bash`

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

After the libraries are buil their PATH exproted. Note that you need to run these two line everytime you restart the container or open a new terminal

```
export CGO_LD_FLAGS=-L/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle/
```
## Running Benchmarks
To run the benchmarks simply run the `main.go` script using the command:

`go run -tags=icicle main.go`