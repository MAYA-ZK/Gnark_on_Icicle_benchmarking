#!/bin/bash

#Set the environment variables
export CGO_LD_FLAGS=-L/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/root/go/pkg/mod/github.com/ingonyama-zk/icicle@v0.1.0/goicicle/

# Define arrays for each parameter
circuit_list=("sha256")
curve_list=("bn254" "bls12_377" "bw6_761")
GPU_Acc=(false true)
n=10

# Save the current value of CUDA_VISIBLE_DEVICES
old_cuda_visible_devices=$CUDA_VISIBLE_DEVICES

# Iterate over all combinations of parameters
for circuit in "${circuit_list[@]}"; do
    for curve in "${curve_list[@]}"; do
        for acc in "${GPU_Acc[@]}"; do
            if [ "$acc" == true ]; then
                go run -tags=icicle,custom_const main.go -curve "$curve" -circuit "$circuit" -GPU_Acc -n "$n"
            else
                go run -tags=icicle,custom_const main.go -curve "$curve" -circuit="$circuit" -n "$n"
            fi
        done
    done
done