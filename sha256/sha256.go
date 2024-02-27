package sha256

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/consensys/gnark/std/hash/sha2"
	"github.com/consensys/gnark/std/math/uints"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"github.com/consensys/gnark/logger"

	"github.com/rs/zerolog"

	"gnark_on_icicle/benchmark"
	"gnark_on_icicle/constants"
	"gnark_on_icicle/gpu"
)

/* Helper functions */

func convert_hash_bytes_to_gnarkU8(hash_byte_arr [32]byte) [32]uints.U8 {
	var hash_gnarkU8_arr [32]uints.U8

	for i, b := range hash_byte_arr {
		hash_gnarkU8_arr[i] = uints.NewU8(uint8(b))
	}

	return hash_gnarkU8_arr
}

func convert_preimage_bytes_to_gnarkU8(preimage_byte_arr [constants.PREIMAGE_SIZE]byte) [constants.PREIMAGE_SIZE]uints.U8 {
	var preimage_gnarkU8_arr [constants.PREIMAGE_SIZE]uints.U8

	for i, b := range preimage_byte_arr {
		preimage_gnarkU8_arr[i] = uints.NewU8(uint8(b))
	}

	return preimage_gnarkU8_arr
}

func Gen_rand_inputs(n int) ([][32]byte, [][constants.PREIMAGE_SIZE]byte, error) {
	rand_hashes := make([][32]byte, n)
	rand_preimages := make([][constants.PREIMAGE_SIZE]byte, n)

	for i := 0; i < n; i++ {
		// Generate random bytes
		randomBytes := make([]byte, constants.PREIMAGE_SIZE)
		_, err := rand.Read(randomBytes)
		if err != nil {
			return nil, nil, err
		}

		// Convert random bytes to hex string
		rand_preimages[i] = [constants.PREIMAGE_SIZE]byte(randomBytes)

		// Calculate SHA256 hash
		rand_hashes[i] = sha256.Sum256(randomBytes)
	}

	return rand_hashes, rand_preimages, nil
}

// Function to read file and extract hashes and preimages
func Parse_file(file_path string) ([][32]byte, [][constants.PREIMAGE_SIZE]byte, error) {
	// Open the file
	file, err := os.Open(file_path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var hashes [][32]byte
	var preimages [][constants.PREIMAGE_SIZE]byte

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
		if len(parts[1]) != constants.PREIMAGE_SIZE*2 {
			return nil, nil, fmt.Errorf("invaild preimage not %d bytes at line %d", constants.PREIMAGE_SIZE, line_num)
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
		preimages = append(preimages, [constants.PREIMAGE_SIZE]byte(preimage_bytes))
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return hashes, preimages, nil
}

// Circuit defines a pre-image knowledge proof
// SHA256(secret PreImage) = public Hash
type SHA256Circuit struct {
	PreImage [constants.PREIMAGE_SIZE]uints.U8
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

func Benchmark(curve_id ecc.ID, GPU_Acc bool, hashes [][32]byte, preimages [][constants.PREIMAGE_SIZE]byte) error {

	// Check if we have the same number of hashes and preimages
	if len(hashes) != len(preimages) {
		fmt.Println("The number of hashes and pre-images are not equal. Please check your input!")
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
	outp.Circuit = "sha256"
	outp.Num_runs = len(hashes)
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

	var SHA256_circuit SHA256Circuit
	scalarfield := curve_id.ScalarField()
	// Keep track of the beginning and end time of each step
	outp.Start_arith = time.Now()
	// compiles our circuit into a R1CS
	ccs, _ := frontend.Compile(scalarfield, r1cs.NewBuilder, &SHA256_circuit)
	outp.Nb_constraints = ccs.GetNbConstraints()
	outp.End_arith = time.Now()

	// groth16 zkSNARK: Setup
	fmt.Println("Running setup...")
	outp.Start_setup = time.Now()
	pk, vk, _ := groth16.Setup(ccs)
	outp.End_setup = time.Now()

	for i := 0; i < len(hashes); i++ {
		fmt.Printf("Benchmark run %d/%d\n", i+1, len(hashes))
		//fmt.Println("Hash : ", hex.EncodeToString(hashes[i][:]))
		//fmt.Println("Pre-image : ", hex.EncodeToString(preimages[i][:]))
		// Circuit assignement
		assignment := SHA256Circuit{PreImage: convert_preimage_bytes_to_gnarkU8(preimages[i]), Hash: convert_hash_bytes_to_gnarkU8(hashes[i])}
		// Witness genration
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
