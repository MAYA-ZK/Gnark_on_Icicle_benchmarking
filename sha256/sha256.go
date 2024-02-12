package sha256

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/consensys/gnark/std/hash/sha2"
	"github.com/consensys/gnark/std/math/uints"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const PREIMAGE_SIZE = 32

/* Helper functions */

func convert_hash_bytes_to_gnarkU8(hash_byte_arr [32]byte) [32]uints.U8 {
	var hash_gnarkU8_arr [32]uints.U8

	for i, b := range hash_byte_arr {
		hash_gnarkU8_arr[i] = uints.NewU8(uint8(b))
	}

	return hash_gnarkU8_arr
}

func convert_preimage_bytes_to_gnarkU8(preimage_byte_arr [PREIMAGE_SIZE]byte) [PREIMAGE_SIZE]uints.U8 {
	var preimage_gnarkU8_arr [PREIMAGE_SIZE]uints.U8

	for i, b := range preimage_byte_arr {
		preimage_gnarkU8_arr[i] = uints.NewU8(uint8(b))
	}

	return preimage_gnarkU8_arr
}

func Gen_rand_inputs(n int) ([][32]byte, [][PREIMAGE_SIZE]byte, error) {
	rand_hashes := make([][32]byte, n)
	rand_preimages := make([][PREIMAGE_SIZE]byte, n)

	for i := 0; i < n; i++ {
		// Generate random bytes
		randomBytes := make([]byte, PREIMAGE_SIZE)
		_, err := rand.Read(randomBytes)
		if err != nil {
			return nil, nil, err
		}

		// Convert random bytes to hex string
		rand_preimages[i] = [PREIMAGE_SIZE]byte(randomBytes)

		// Calculate SHA256 hash
		rand_hashes[i] = sha256.Sum256(randomBytes)
	}

	return rand_hashes, rand_preimages, nil
}

// Function to read file and extract hashes and preimages
func Parse_file(file_path string) ([][32]byte, [][PREIMAGE_SIZE]byte, error) {
	// Open the file
	file, err := os.Open(file_path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var hashes [][32]byte
	var preimages [][PREIMAGE_SIZE]byte

	scanner := bufio.NewScanner(file)
	line_num := 0
	for scanner.Scan() {
		line := scanner.Text()
		line_num++
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid line format: %s", line)
		}
		// Check if the hashes and the preimages are the correct length
		if len(parts[0]) != 32*2 {
			return nil, nil, fmt.Errorf("invaild hash not 32 bytes at line %d", line_num)
		}
		if len(parts[1]) != PREIMAGE_SIZE*2 {
			return nil, nil, fmt.Errorf("invaild preimage not %d bytes at line %d", PREIMAGE_SIZE, line_num)
		}
		// Decode the hashes and preimages from hexadecimal strings into byte arrays
		hash_bytes, err := hex.DecodeString(parts[0])
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding hash at line %d: %v", line_num, err)
		}

		preimage_bytes, err := hex.DecodeString(parts[1])
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding pre-image at line %d: %v", line_num, err)
		}

		hashes = append(hashes, [32]byte(hash_bytes))
		preimages = append(preimages, [PREIMAGE_SIZE]byte(preimage_bytes))
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return hashes, preimages, nil
}

// Circuit defines a pre-image knowledge proof
// SHA256(secret PreImage) = public Hash
type SHA256Circuit struct {
	PreImage [PREIMAGE_SIZE]uints.U8
	Hash     [32]uints.U8 `gnark:",public"`
}

// Define declares the circuit's constraints
// Hash = sha256(PreImage)
func (circuit *SHA256Circuit) Define(api frontend.API) error {
	h, err := sha2.New(api)
	if err != nil {
		return err
	}
	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	h.Write(circuit.PreImage[:])
	res := h.Sum()
	if len(res) != 32 {
		return fmt.Errorf("not 32 bytes")
	}
	for i := range circuit.Hash {
		uapi.ByteAssertEq(circuit.Hash[i], res[i])
	}
	return nil
}

func Benchmark(scalarfield *big.Int, GPU_Acc bool, hashes [][32]byte, preimages [][PREIMAGE_SIZE]byte) error {

	// Check if we have the same number of hashes and preimages

	if len(hashes) != len(preimages) {
		fmt.Println("The number of hashes and pre-images are not equal. Please check your input!")
		return nil
	}

	var SHA256_circuit SHA256Circuit

	// compiles our circuit into a R1CS
	ccs, _ := frontend.Compile(scalarfield, r1cs.NewBuilder, &SHA256_circuit)

	// groth16 zkSNARK: Setup
	pk, vk, _ := groth16.Setup(ccs)

	// witness definition
	var i int
	for i = 0; i < len(hashes); i++ {

		fmt.Println("Hash : ", hex.EncodeToString(hashes[i][:]))
		fmt.Println("Pre-image : ", hex.EncodeToString(preimages[i][:]))

		assignment := SHA256Circuit{PreImage: convert_preimage_bytes_to_gnarkU8(preimages[i]), Hash: convert_hash_bytes_to_gnarkU8(hashes[i])}
		witness, _ := frontend.NewWitness(&assignment, scalarfield)
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
