package exponentiate

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/math/bits"
)

// Number of bits of exponent
const EXP_BITSIZE = 8
const X_SIZE = 16

func Gen_rand_inputs(n int) ([]*big.Int, []*big.Int, []uint8, error) {
	x := make([]*big.Int, n)
	y := make([]*big.Int, n)
	e := make([]uint8, n)

	for i := 0; i < n; i++ {
		// Generate random number
		buf_x := make([]byte, X_SIZE+7)
		// Read random bytes into the buffer
		_, err := rand.Read(buf_x[:])
		if err != nil {
			return nil, nil, nil, err
		}
		// convert random bytes into big.Int
		x[i] = new(big.Int).SetBytes(buf_x[:])

		// Generate random exponent
		var buf_e [1]byte
		// Read random bytes into the buffer
		_, err = rand.Read(buf_e[:])
		if err != nil {
			return nil, nil, nil, err
		}
		// Convert random bytes into big.Int
		e[i] = uint8(buf_e[0])

		// Calculate y value
		y[i] = new(big.Int).Exp(x[i], big.NewInt(int64(e[i])), nil)
	}

	return x, y, e, nil
}

// Function to read file and extract hashes and preimages
func Parse_file(file_path string) ([]*big.Int, []*big.Int, []uint8, error) {
	// Open the file
	file, err := os.Open(file_path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer file.Close()

	var x []*big.Int
	var y []*big.Int
	var e []uint8

	scanner := bufio.NewScanner(file)
	line_num := 0
	for scanner.Scan() {
		line := scanner.Text()
		line_num++
		parts := strings.Split(line, " ")
		if len(parts) != 3 {
			return nil, nil, nil, fmt.Errorf("invalid line format: %s", line)
		}
		// Read x value
		x_val, succ := new(big.Int).SetString(parts[0], 10)
		if !succ {
			return nil, nil, nil, fmt.Errorf("error decoding x at line %d: Failed to convert string to big.Int", line_num)
		}
		// Read y value
		y_val, succ := new(big.Int).SetString(parts[1], 10)
		if !succ {
			return nil, nil, nil, fmt.Errorf("error decoding y at line %d: Failed to convert string to big.Int", line_num)
		}
		// Read e value
		e_val, err := strconv.ParseUint(parts[2], 10, 8)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error decoding e at line %d: %v", line_num, err)
		}
		// Append the values to the slices
		x = append(x, x_val)
		y = append(y, y_val)
		e = append(e, uint8(e_val))
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}

	return x, y, e, nil
}

type ExpCircuit struct {
	// tagging a variable is optional
	// default uses variable name and secret visibility.
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable `gnark:",public"`

	E frontend.Variable
}

// Define declares the circuit's constraints
// y == x**e
func (circuit *ExpCircuit) Define(api frontend.API) error {

	// specify constraints
	output := frontend.Variable(1)
	bits := bits.ToBinary(api, circuit.E, bits.WithNbDigits(EXP_BITSIZE))

	for i := 0; i < len(bits); i++ {
		if i != 0 {
			output = api.Mul(output, output)
		}
		multiply := api.Mul(output, circuit.X)
		output = api.Select(bits[len(bits)-1-i], multiply, output)

	}

	api.AssertIsEqual(circuit.Y, output)

	return nil
}

func Benchmark(ScalarField *big.Int, GPU_Acc bool, x []*big.Int, y []*big.Int, e []uint8) error {
	if len(x) != len(y) || len(x) != len(e) || len(y) != len(e) {
		fmt.Println("The number of x and y, x and e or y and e values are not equal. Please check your input!")
		return nil
	}
	// compiles our circuit into a R1CS
	var circuit ExpCircuit
	ccs, _ := frontend.Compile(ScalarField, r1cs.NewBuilder, &circuit)

	// groth16 zkSNARK: Setup
	pk, vk, _ := groth16.Setup(ccs)

	var i int
	for i = 0; i < len(x); i++ {
		fmt.Println("x : ", x[i].String())
		fmt.Println("y : ", y[i].String())
		fmt.Println("e : ", e[i])
		// witness definition
		assignment := ExpCircuit{X: x[i], Y: y[i], E: e[i]}
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
