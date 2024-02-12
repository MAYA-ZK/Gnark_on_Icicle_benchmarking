package cubic

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const X_SIZE = 8

func Gen_rand_inputs(n int) ([]*big.Int, []*big.Int, error) {
	x := make([]*big.Int, n)
	y := make([]*big.Int, n)

	for i := 0; i < n; i++ {
		// Generate random number
		var buf [8]byte
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

func Benchmark(ScalarField *big.Int, GPU_Acc bool, x []*big.Int, y []*big.Int) error {
	if len(x) != len(y) {
		fmt.Println("The number of x and y values are not equal. Please check your input!")
		return nil
	}

	// compiles our circuit into a R1CS
	var circuit CubicCircuit
	ccs, _ := frontend.Compile(ScalarField, r1cs.NewBuilder, &circuit)

	// groth16 zkSNARK: Setup
	pk, vk, _ := groth16.Setup(ccs)

	var i int
	for i = 0; i < len(x); i++ {
		fmt.Println("x : ", x[i].String())
		fmt.Println("y : ", y[i].String())
		// witness definition
		assignment := CubicCircuit{X: *x[i], Y: *y[i]}
		witness, _ := frontend.NewWitness(&assignment, ScalarField)
		publicWitness, _ := witness.Public()

		// groth16: Prove & Verify
		var proof groth16.Proof
		var err error
		if GPU_Acc {
			proof, err = groth16.Prove(ccs, pk, witness, backend.WithIcicleAcceleration())
		} else {
			proof, err = groth16.Prove(ccs, pk, witness)
		}

		if err != nil {
			fmt.Println(err)
		}

		err = groth16.Verify(proof, vk, publicWitness)
		if err == nil {
			fmt.Println("Proof is valid!")
		} else {
			fmt.Println("Proof is invalid: ", err)
		}
	}

	return nil
}
