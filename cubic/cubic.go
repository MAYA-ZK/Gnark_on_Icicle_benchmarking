package cubic

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"gnark_on_icicle/benchmark"
	"gnark_on_icicle/constants"
	"gnark_on_icicle/gpu"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/logger"
	"github.com/rs/zerolog"
)

func Gen_rand_inputs(n int) ([]*big.Int, []*big.Int, error) {
	x := make([]*big.Int, n)
	y := make([]*big.Int, n)

	for i := 0; i < n; i++ {
		// Generate random number
		var buf [constants.X_SIZE_CUBIC]byte
		// Read random bytes into the buffer
		_, err := rand.Read(buf[:])
		if err != nil {
			return nil, nil, err
		}

		// Convert random bytes to Int64
		x[i] = new(big.Int).SetBytes(buf[:])
		// Divide x[i] by 2 so it's technically 63-bit long in magintude
		x[i] = new(big.Int).Div(x[i], big.NewInt(2))
		// This last line only generates random numbers. To accomodate for negative numbers as well
		// we look at the first bit of the first byte and use its as a random bit to generate the sign
		if uint8(buf[0])%2 == 1 {
			x[i] = x[i].Neg(x[i])
		}
		// Calculate the corresponding y value
		// x^2
		x_squared_big := new(big.Int).Mul(x[i], x[i])
		// x^3
		x_cube_big := new(big.Int).Mul(x[i], x_squared_big)
		// x^3 + x
		x_sum_big := new(big.Int).Add(x_cube_big, x[i])
		// x^3 + x + 5
		y[i] = new(big.Int).Add(x_sum_big, big.NewInt(5))
	}

	return x, y, nil
}

// Function to read file and extract hashes and preimages
func Parse_file(file_path string) ([]*big.Int, []*big.Int, error) {
	// Open the file
	file, err := os.Open(file_path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var x []*big.Int
	var y []*big.Int

	scanner := bufio.NewScanner(file)
	line_num := 0
	for scanner.Scan() {
		line := scanner.Text()
		line_num++
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid line format: %s", line)
		}
		// Decode the hashes and preimages from hexadecimal strings into byte arrays
		x_bigint, succ := new(big.Int).SetString(parts[0], 10)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding x at line %d: %v", line_num, err)
		}
		// Convert string to big.Int
		y_bigint, succ := new(big.Int).SetString(parts[1], 10)
		if !succ {
			return nil, nil, fmt.Errorf("error decoding y at line %d: Failed to convert string to big.Int", line_num)
		}
		// Append the values to the slices
		x = append(x, x_bigint)
		y = append(y, y_bigint)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return x, y, nil
}

// CubicCircuit defines a simple circuit
// x**3 + x + 5 == y

type CubicCircuit struct {
	// struct tags on a variable is optional
	// default uses variable name and secret visibility.
	X frontend.Variable `gnark:"x"`
	Y frontend.Variable `gnark:",public"`
}

// Define declares the circuit constraints
// x**3 + x + 5 == y

func (circuit *CubicCircuit) Define(api frontend.API) error {
	x3 := api.Mul(circuit.X, circuit.X, circuit.X)
	api.AssertIsEqual(circuit.Y, api.Add(x3, circuit.X, 5))
	return nil
}

func Benchmark(curve_id ecc.ID, GPU_Acc bool, x []*big.Int, y []*big.Int) error {
	if len(x) != len(y) {
		fmt.Println("The number of x and y values are not equal. Please check your input!")
		return nil
	}

	// Create a buffer to store logs
	var buf bytes.Buffer
	// Overtake the gnark logger with another one that outputs to a buffer and the console
	multi := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout}, &buf)
	logger.Set(zerolog.New(multi).With().Timestamp().Logger())
	// Create a variable to save all the values from the benchmark
	var outp benchmark.Benchmark_Output
	// Set the circuit and number of runs
	outp.Circuit = "cubic"
	outp.Num_runs = len(x)
	outp.GPU_Acc = GPU_Acc
	outp.Curve = curve_id.String()

	// Initialize the GPU logging if GPU acceleration is used
	// Create a channel to signal the GPU sampling function to stop
	stop := make(chan struct{})
	// Create a channel to receive the GPU samples
	GPU_samples := make(chan []gpu.GPU_Sample)
	if GPU_Acc {
		// Initilaize NVML
		gpu.Init_NVML()
		// Get the GPU device
		device := gpu.Get_device(0)

		// Start the GPU sampling funciton as a goroutine
		go gpu.GPU_Periodic_Samples(gpu.SAMPLIMG_PERIOD, device, stop, GPU_samples)
	}

	// compiles our circuit into a R1CS
	var circuit CubicCircuit
	scalarfield := curve_id.ScalarField()
	// Keep track of the beginning and end time of each run
	outp.Start_arith = time.Now()
	ccs, _ := frontend.Compile(scalarfield, r1cs.NewBuilder, &circuit)
	outp.End_arith = time.Now()
	// groth16 zkSNARK: Setup
	fmt.Println("Running setup...")
	outp.Start_setup = time.Now()
	pk, vk, _ := groth16.Setup(ccs)
	outp.End_setup = time.Now()

	var i int
	for i = 0; i < len(x); i++ {
		fmt.Printf("Benchmark run %d/%d\n", i+1, len(x))
		//fmt.Println("x : ", x[i].String())
		//fmt.Println("y : ", y[i].String())
		// witness definition
		assignment := CubicCircuit{X: *x[i], Y: *y[i]}
		outp.Start_witness_gen = append(outp.Start_witness_gen, time.Now())
		witness, _ := frontend.NewWitness(&assignment, scalarfield)
		publicWitness, _ := witness.Public()
		outp.End_witness_gen = append(outp.End_witness_gen, time.Now())
		// groth16: Prove & Verify
		var proof groth16.Proof
		var err error
		outp.Start_proof_gen_func = append(outp.Start_proof_gen_func, time.Now())
		if GPU_Acc {
			proof, err = groth16.Prove(ccs, pk, witness, backend.WithIcicleAcceleration())
		} else {
			proof, err = groth16.Prove(ccs, pk, witness)
		}
		outp.End_proof_gen_func = append(outp.End_proof_gen_func, time.Now())
		if err != nil {
			fmt.Println(err)
		}
		outp.Start_proof_ver = append(outp.Start_proof_ver, time.Now())
		err = groth16.Verify(proof, vk, publicWitness)
		outp.End_proof_ver = append(outp.End_proof_ver, time.Now())
		if err == nil {
			fmt.Println("Proof is valid!")
		} else {
			fmt.Println("Proof is invalid: ", err)
		}
		outp.Proof_valid = append(outp.Proof_valid, err == nil)
	}

	if GPU_Acc {
		// Signal the GPU sampling function to stop
		close(stop)

		// Wait for the periodic function to return the result
		outp.GPU_samples = <-GPU_samples
	}
	outp.Dbg_log = buf.String()
	fmt.Println("Compiling benchmark results...")
	benchmark.Compile(outp)

	return nil
}
