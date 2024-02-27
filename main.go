package main

import (
	"flag"
	"fmt"

	"gnark_on_icicle/cubic"
	"gnark_on_icicle/exponentiate"
	"gnark_on_icicle/sha256"

	"github.com/consensys/gnark-crypto/ecc"
)

const MAX_INPUTS = 1000

func benchmark_from_file(circuit string, curve_id ecc.ID, GPU_Acc bool, file_path string) {
	switch circuit {
	case "cubic":
		x, y, err := cubic.Parse_file(file_path)
		if err != nil {
			fmt.Println("Error parsing file: ", err)
			return
		}
		cubic.Benchmark(curve_id, GPU_Acc, x, y)
		break
	case "exponentiate":
		x, y, e, err := exponentiate.Parse_file(file_path)
		if err != nil {
			fmt.Println("Error parsing file: ", err)
			return
		}
		exponentiate.Benchmark(curve_id, GPU_Acc, x, y, e)
		break
	case "sha256":
		hashes, preimages, err := sha256.Parse_file(file_path)
		if err != nil {
			fmt.Println("Error parsing file: ", err)
			return
		}
		sha256.Benchmark(curve_id, GPU_Acc, hashes, preimages)
		break
	default:
		fmt.Println("Circuit ", circuit, " unknown. The program will benchmark the sha256 circuit...")
		hashes, preimages, err := sha256.Parse_file(file_path)
		if err != nil {
			fmt.Println("Error generating random inputs: ", err)
			return
		}
		sha256.Benchmark(curve_id, GPU_Acc, hashes, preimages)
		break
	}
	fmt.Println("Benchmark ran successfully. Exiting...")
}
func benchmark_rand_vals(circuit string, curve_id ecc.ID, GPU_Acc bool, n int) {
	switch circuit {
	case "cubic":
		x, y, err := cubic.Gen_rand_inputs(n)
		if err != nil {
			fmt.Println("Error : ", err)
			return
		}
		cubic.Benchmark(curve_id, GPU_Acc, x, y)
		break
	case "exponentiate":
		x, y, e, err := exponentiate.Gen_rand_inputs(n)
		if err != nil {
			fmt.Println("Error : ", err)
			return
		}
		exponentiate.Benchmark(curve_id, GPU_Acc, x, y, e)
		break
	case "sha256":
		hashes, preimages, err := sha256.Gen_rand_inputs(n)
		if err != nil {
			fmt.Println("Error : ", err)
			return
		}
		sha256.Benchmark(curve_id, GPU_Acc, hashes, preimages)
		break
	default:
		fmt.Println("Circuit ", circuit, " unknown. The program will benchmark the sha256 circuit...")
		hashes, preimages, err := sha256.Gen_rand_inputs(n)
		if err != nil {
			fmt.Println("Error generating random inputs: ", err)
			return
		}
		sha256.Benchmark(curve_id, GPU_Acc, hashes, preimages)
		break
	}
	fmt.Println("Benchmark ran successfully. Exiting...")
}

func main() {
	// Parse arguments
	var curve string
	var circuit string
	var GPU_Acc bool
	var n int
	var file_path string

	fmt.Println("Parsing arguments...")
	flag.StringVar(&curve, "curve", "bn254", "Specify the curve")
	flag.StringVar(&circuit, "circuit", "sha256", "Specify the circuit to benchmark")
	flag.BoolVar(&GPU_Acc, "GPU_Acc", false, "Enable GPU acceleration")
	flag.IntVar(&n, "n", 0, "Number of random inputs to run the benchmark on")
	flag.StringVar(&file_path, "file_path", "", "Path to file containing pre-determined inputs seperated by a space")

	flag.Parse()
	fmt.Println("Benchmark parameters: ")
	fmt.Println("\t-curve:", curve)
	fmt.Println("\t-circuit:", circuit)
	fmt.Println("\t-GPU Acceleration: ", GPU_Acc)
	// Set the scalar field depending on the choice of the curve
	var curve_id ecc.ID
	switch curve {
	case "bn254":
		curve_id = ecc.BN254
		break
	case "bls12_377":
		curve_id = ecc.BLS12_377
		break
	case "bls12_381":
		curve_id = ecc.BLS12_381
		break
	case "bw6_761":
		curve_id = ecc.BW6_761
		break
	default:
		fmt.Println("Curve", curve, "unknown. The benchmark will use the bn254 curve...")
		curve_id = ecc.BN254
		break
	}
	// Get the inputs for the circuit
	if file_path != "" {
		benchmark_from_file(circuit, curve_id, GPU_Acc, file_path)
		return
	} else if n != 0 {
		if n < 0 {
			fmt.Println("n does not accept negative numbers. Please give a positive number")
			return
		}
		if n > MAX_INPUTS {
			fmt.Printf("The maximum number of inputs is %d. Pleas a give a smaller number for n\n", MAX_INPUTS)
			return
		}
		benchmark_rand_vals(circuit, curve_id, GPU_Acc, n)
	} else {
		fmt.Println("No inputs were detected, the program will be running with 10 random inputs...")
		benchmark_rand_vals(circuit, curve_id, GPU_Acc, 10)
		return
	}

}
