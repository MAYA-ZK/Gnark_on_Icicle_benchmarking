package benchmark

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"gnark_on_icicle/gpu"
)

type Log_entry struct {
	Level          string  `json:"level"`
	Time           string  `json:"time"`
	Message        string  `json:"message"`
	Curve          string  `json:"curve"`
	Nb_constraints int     `json:"nbConstraints"`
	Acc            string  `json:"acceleration"`
	Backend        string  `json:"backend"`
	Duration       float64 `json:"took"`
}

type Benchmark_Output struct {
	Start_arith          time.Time
	End_arith            time.Time
	Start_setup          time.Time
	End_setup            time.Time
	Start_witness_gen    []time.Time
	End_witness_gen      []time.Time
	Start_sol_gen        []time.Time
	End_sol_gen          []time.Time
	Start_proof_gen      []time.Time
	End_proof_gen        []time.Time
	Start_proof_gen_func []time.Time
	End_proof_gen_func   []time.Time
	Start_proof_ver      []time.Time
	End_proof_ver        []time.Time
	Proof_valid          []bool
	GPU_samples          []gpu.GPU_Sample
	Dbg_log              string

	Circuit        string
	Curve          string
	GPU_Acc        bool
	Num_runs       int
	Nb_constraints int
	Cubic_x        []*big.Int
	Cubic_y        []*big.Int
	Exponentiate_x []*big.Int
	Exponentiate_y []*big.Int
	Exponentiate_e []uint8
}

type benchmark_params struct {
	Circuit        string `json:"Circuit"`
	Curve          string `json:"Curve"`
	Acc            string `json:"Accelerator"`
	Num_runs       int    `json:"Number of runs"`
	Nb_constraints int    `json:"Number of constraints"`
	Arith_dur      int    `json:"Arithmatization duration"`
	Setup_dur      int    `json:"Setup duration"`
}

func Compile(outp Benchmark_Output) error {
	// Check if the output folder exists
	_, err := os.Stat("./output")
	if err != nil {
		if os.IsNotExist(err) {
			// If the folder does not exist, create it
			err := os.MkdirAll("./output", 0755)
			if err != nil {
				return err
			}
		}
	}
	// Use the timestamp of the start fo the first arithmatization to name the folder where all the benchmark results are stored
	var outp_folderpath string
	i := 0
	for {
		outp_folderpath = fmt.Sprintf("./output/benchmark-%d", i)
		_, err := os.Stat(outp_folderpath)
		if os.IsNotExist(err) {
			// If the folder does not exist, create it
			err := os.MkdirAll(outp_folderpath, 0755)
			if err != nil {
				return err
			}
			break
		}
		i++
	}

	// Parse the debug logs
	var log_entries []Log_entry

	decoder := json.NewDecoder(bytes.NewBufferString(outp.Dbg_log))
	for {
		var entry Log_entry
		if err := decoder.Decode(&entry); err != nil {
			break // Stop decoding on error or end of input
		}
		log_entries = append(log_entries, entry)
	}
	// Add data from the logs
	outp.Curve = log_entries[5].Curve
	// Create a JSON file to save the benchmark parameters
	bench_params := benchmark_params{
		Circuit:        outp.Circuit,
		Curve:          outp.Curve,
		Acc:            map[bool]string{true: "GPU", false: "CPU"}[outp.GPU_Acc],
		Num_runs:       outp.Num_runs,
		Nb_constraints: outp.Nb_constraints,
		Arith_dur:      int(outp.End_arith.Sub(outp.Start_arith).Milliseconds()),
		Setup_dur:      int(outp.End_setup.Sub(outp.Start_setup).Milliseconds()),
	}
	// Marshal the data into JSON format
	data_json, err := json.MarshalIndent(bench_params, "", "    ")
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}
	bench_params_filepath := fmt.Sprintf("%s/benchmark_parameters.json", outp_folderpath)

	write_JSON_file(bench_params_filepath, data_json)

	// Create a CSV file to save the banchmarking results
	var data_csv [][]string
	// Create the header
	data_csv = append(data_csv, []string{"Run number", "Witness generation", "Solution generation", "Proof generation", "Proof generation (full function)", "Proof verification", "Full run", "Valid proof"})
	outp.End_proof_gen = make([]time.Time, outp.Num_runs)
	// We assume that the function groth16.prove ended at the same time the proof generation ended
	copy(outp.End_proof_gen, outp.End_proof_gen_func)
	// Create variables to keep track of the cumultative duration of each step to calculate the average later for the summary
	var witness_gen_dur_cumul, sol_gen_dur_cumul, proof_gen_dur_cumul, proof_gen_func_dur_cumul, proof_ver_dur_cumul, full_run_dur_cumul int64
	for i := 0; i < outp.Num_runs; i++ {
		// The function groth16.prove performs both the solution generation and the proof generation there fore we need to extract
		// timings from the gnark logs
		var sol_gen_dur, proof_gen_dur time.Duration
		if outp.GPU_Acc {
			if outp.Proof_valid[i] {
				sol_gen_dur = time.Duration(int(log_entries[4+3*i].Duration)) * time.Millisecond
				proof_gen_dur = time.Duration(int(log_entries[5+3*i].Duration)) * time.Millisecond
			} else {
				sol_gen_dur = time.Duration(int(log_entries[4+2*i].Duration)) * time.Millisecond
				proof_gen_dur = time.Duration(int(log_entries[5+2*i].Duration)) * time.Millisecond
			}
		} else {
			if outp.Proof_valid[i] {
				sol_gen_dur = time.Duration(int(log_entries[3+3*i].Duration)) * time.Millisecond
				proof_gen_dur = time.Duration(int(log_entries[4+3*i].Duration)) * time.Millisecond
			} else {
				sol_gen_dur = time.Duration(int(log_entries[3+2*i].Duration)) * time.Millisecond
				proof_gen_dur = time.Duration(int(log_entries[4+2*i].Duration)) * time.Millisecond
			}
		}
		// Subtract the proof generation duration from the end time of the function to get the start time of the proof generation
		// Subtract the solution generation duration from the start time of the proof generation to ge the start time of the solution generation
		outp.Start_proof_gen = append(outp.Start_proof_gen, outp.End_proof_gen[i].Add(-proof_gen_dur))
		outp.End_sol_gen = append(outp.End_sol_gen, outp.End_proof_gen[i].Add(-proof_gen_dur))
		outp.Start_sol_gen = append(outp.Start_sol_gen, outp.End_proof_gen[i].Add(-proof_gen_dur).Add(-sol_gen_dur))
		// Calculate the duration of each step and keep track of the cumultative sum
		witness_gen_dur := outp.End_witness_gen[i].Sub(outp.Start_witness_gen[i])
		witness_gen_dur_cumul += witness_gen_dur.Milliseconds()
		sol_gen_dur_cumul += sol_gen_dur.Milliseconds()
		proof_gen_dur_cumul += proof_gen_dur.Milliseconds()
		proof_gen_func_dur := outp.End_proof_gen_func[i].Sub(outp.Start_proof_gen_func[i])
		proof_gen_func_dur_cumul += proof_gen_func_dur.Milliseconds()
		proof_ver_dur := outp.End_proof_ver[i].Sub(outp.Start_proof_ver[i])
		proof_ver_dur_cumul += proof_ver_dur.Milliseconds()
		full_run_dur := outp.End_proof_ver[i].Sub(outp.Start_witness_gen[i])
		full_run_dur_cumul += full_run_dur.Milliseconds()

		// Create the line to add to the csv
		run_number_str := strconv.FormatInt(int64(i), 10)
		witness_gen_dur_str := strconv.FormatInt(witness_gen_dur.Milliseconds(), 10)
		sol_gen_dur_str := strconv.FormatInt(sol_gen_dur.Milliseconds(), 10)
		proof_gen_dur_str := strconv.FormatInt(proof_gen_dur.Milliseconds(), 10)
		proof_gen_func_dur_str := strconv.FormatInt(proof_gen_func_dur.Milliseconds(), 10)
		proof_ver_dur_str := strconv.FormatInt(proof_ver_dur.Milliseconds(), 10)
		full_run_dur_str := strconv.FormatInt(full_run_dur.Milliseconds(), 10)

		data_csv = append(data_csv, []string{run_number_str, witness_gen_dur_str, sol_gen_dur_str, proof_gen_dur_str, proof_gen_func_dur_str,
			proof_ver_dur_str, full_run_dur_str, strconv.FormatBool(outp.Proof_valid[i])})

	}
	benchmark_res_filepath := fmt.Sprintf("%s/benchmark_results.csv", outp_folderpath)
	write_CSV_file(benchmark_res_filepath, data_csv)

	// Write the summary of the benchmark with the averages of each step's duration
	data_csv = data_csv[:0]
	// Create the header
	data_csv = append(data_csv, []string{"Arithmitization", "Setup", "Avg witness generation", "Avg solution generation", "Avg proof generation",
		"Avg proof generation function", "Avg full run"})
	data_csv = append(data_csv, []string{strconv.FormatInt(int64(outp.End_arith.Sub(outp.Start_arith).Milliseconds()), 10),
		strconv.FormatInt(int64(outp.End_setup.Sub(outp.Start_setup).Milliseconds()), 10),
		strconv.FormatInt(int64(witness_gen_dur_cumul/int64(outp.Num_runs)), 10),
		strconv.FormatInt(int64(sol_gen_dur_cumul/int64(outp.Num_runs)), 10),
		strconv.FormatInt(int64(proof_gen_dur_cumul/int64(outp.Num_runs)), 10),
		strconv.FormatInt(int64(proof_gen_func_dur_cumul/int64(outp.Num_runs)), 10),
		strconv.FormatInt(int64(full_run_dur_cumul/int64(outp.Num_runs)), 10)})
	benchmark_summary_filepath := fmt.Sprintf("%s/benchmark_summary.csv", outp_folderpath)
	write_CSV_file(benchmark_summary_filepath, data_csv)

	// Log GPU stats if GPU acceleration was used
	if outp.GPU_Acc {
		// First we extract the GPU samples during the runs (arithmatization and setup not included)
		timestamps, gpu_util, gpu_mem, gpu_pow := GPU_samples_slice(outp.GPU_samples, outp.Start_witness_gen[0], outp.End_proof_ver[outp.Num_runs-1])
		// Integrate the GPU stats during all the runs to be able to easily calculate the averages later
		_, gpu_util_integral := integrate(timestamps, gpu_util)
		_, gpu_mem_integral := integrate(timestamps, gpu_mem)
		_, gpu_pow_integral := integrate(timestamps, gpu_pow)
		// Write GPU stats in a csv file
		data_csv = data_csv[:0]
		// Create the header
		data_csv = append(data_csv, []string{"Run number", "Run duration", "GPU util", "GPU mem avg", "GPU mem peak ", "GPU power avg", "GPU power peak", "GPU energy"})
		for i := 0; i < outp.Num_runs; i++ {
			// Run duration
			run_dur := outp.End_proof_ver[i].Sub(outp.Start_witness_gen[i])
			// Get the max value of each metric
			max_gpu_util := get_max(timestamps, gpu_util, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			max_gpu_mem := get_max(timestamps, gpu_mem, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			max_gpu_pow := get_max(timestamps, gpu_pow, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			max_gpu_util_str := strconv.FormatUint(max_gpu_util, 10)
			max_gpu_mem_str := strconv.FormatFloat(float64(max_gpu_mem)/(1024.0*1024.0), 'f', 2, 64)
			max_gpu_pow_str := strconv.FormatUint(max_gpu_pow, 10)
			// Get the average of each metric
			avg_gpu_util := get_avg(timestamps, gpu_util_integral, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			avg_gpu_mem := get_avg(timestamps, gpu_mem_integral, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			avg_gpu_pow := get_avg(timestamps, gpu_pow_integral, outp.Start_witness_gen[i], outp.End_proof_ver[i])
			// The integration happens on the microseconds scale therefore to get the average we divide by the duration in microseconds
			run_num_str := strconv.FormatInt(int64(i), 10)
			run_dur_str := strconv.FormatInt(outp.End_sol_gen[i].Sub(outp.Start_sol_gen[i]).Milliseconds(), 10)
			avg_gpu_util_str := strconv.FormatFloat(avg_gpu_util, 'f', 2, 64)
			avg_gpu_mem_str := strconv.FormatFloat(avg_gpu_mem/(1024.0*1024.0), 'f', 3, 64)
			avg_gpu_pow_str := strconv.FormatFloat(avg_gpu_pow, 'f', 3, 64)
			gpu_energy_str := strconv.FormatFloat((avg_gpu_pow*float64(run_dur.Microseconds()))/1000000.0, 'f', 3, 64) // divide by 1000000 to ge the results in milliwattsecond
			data_csv = append(data_csv, []string{run_num_str, run_dur_str, avg_gpu_util_str, max_gpu_util_str, avg_gpu_mem_str, max_gpu_mem_str, avg_gpu_pow_str, max_gpu_pow_str, gpu_energy_str})
		}
		gpu_stats_filepath := fmt.Sprintf("%s/gpu_stats.csv", outp_folderpath)
		write_CSV_file(gpu_stats_filepath, data_csv)
		// Do the same thing to write the histogram of the gpu stats throughout the benchmark runs
		data_csv = data_csv[:0]
		// Create the header
		data_csv = append(data_csv, []string{"t", "GPU util", "GPU mem", "GPU power"})
		for i := 0; i < len(timestamps); i++ {
			t_str := strconv.FormatFloat(float64(timestamps[i].Sub(timestamps[0]).Microseconds())/100.0, 'f', 3, 64)
			gpu_util_str := strconv.FormatUint(gpu_util[i], 10)
			gpu_mem_str := strconv.FormatFloat(float64(gpu_mem[i])/(1024.0*1024.0), 'f', 3, 64)
			gpu_pow_str := strconv.FormatUint(gpu_pow[i], 10)
			data_csv = append(data_csv, []string{t_str, gpu_util_str, gpu_mem_str, gpu_pow_str})
		}
		gpu_samples_filepath := fmt.Sprintf("%s/gpu_samples.csv", outp_folderpath)
		write_CSV_file(gpu_samples_filepath, data_csv)
		// Do the same to write the time stamps of each steps from each run to be able to interpret the histpgramm of the GPU samples
		data_csv = data_csv[:0]
		// Create the header
		data_csv = append(data_csv, []string{"Run number", "Witness gen start", "Witness gen end", "Solution gen start", "Solution gen end",
			"Proof gen start", "Proof gen end", "Proof gen func start", "Proof gen func end", "Proof ver start", "Proof ver end"})
		for i := 0; i < outp.Num_runs; i++ {
			run_num_str := strconv.FormatInt(int64(i), 10)
			witness_gen_start_str := strconv.FormatFloat(float64(outp.Start_witness_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			witness_gen_end_str := strconv.FormatFloat(float64(outp.End_witness_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			sol_gen_start_str := strconv.FormatFloat(float64(outp.Start_sol_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			sol_gen_end_str := strconv.FormatFloat(float64(outp.End_sol_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_gen_start_str := strconv.FormatFloat(float64(outp.Start_proof_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_gen_end_str := strconv.FormatFloat(float64(outp.End_proof_gen[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_gen_func_start_str := strconv.FormatFloat(float64(outp.Start_proof_gen_func[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_gen_func_end_str := strconv.FormatFloat(float64(outp.End_proof_gen_func[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_ver_start_str := strconv.FormatFloat(float64(outp.Start_proof_ver[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)
			proof_ver_end_str := strconv.FormatFloat(float64(outp.End_proof_ver[i].Sub(outp.Start_witness_gen[0]).Microseconds())/1000.0, 'f', 3, 64)

			data_csv = append(data_csv, []string{run_num_str, witness_gen_start_str, witness_gen_end_str, sol_gen_start_str, sol_gen_end_str, proof_gen_start_str,
				proof_gen_end_str, proof_gen_func_start_str, proof_gen_func_end_str, proof_ver_start_str, proof_ver_end_str})
		}
		timestamps_filepath := fmt.Sprintf("%s/timestamps.csv", outp_folderpath)
		write_CSV_file(timestamps_filepath, data_csv)
	}
	fmt.Println("Benchmark results written in", outp_folderpath)

	return nil
}

func GPU_samples_slice(GPU_samples []gpu.GPU_Sample, start time.Time, end time.Time) ([]time.Time, []uint64, []uint64, []uint64) {
	var timestamps []time.Time
	var gpu_util []uint64
	var gpu_mem []uint64
	var gpu_pow []uint64

	for _, sample := range GPU_samples {
		if sample.Timestamp.After(start) && sample.Timestamp.Before(end) {
			timestamps = append(timestamps, sample.Timestamp)
			gpu_util = append(gpu_util, uint64(sample.Util.Gpu))
			gpu_mem = append(gpu_mem, sample.Mem.Used)
			gpu_pow = append(gpu_pow, uint64(sample.Pow))
		}
	}
	return timestamps, gpu_util, gpu_mem, gpu_pow
}

func integrate(x []time.Time, y []uint64) (float64, []float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, []float64{} // cannot integrate
	}

	var integral float64
	var cumul_intergral []float64
	integral = 0

	for i := 0; i < len(x)-1; i++ {
		// Calculate the width of the interval
		duration := x[i+1].Sub(x[i]).Microseconds()

		// Calculate the average height of the interval
		avgY := float64(y[i]+y[i+1]) / 2.0

		// Add the area of the trapezoid to the integral
		integral += avgY * float64(duration)
		// Append the result to the cumultative integral
		cumul_intergral = append(cumul_intergral, integral)
	}
	// Add the last value of the integral to the cumultative integral so that it has the same length as the timestamp
	cumul_intergral = append(cumul_intergral, integral)

	return integral, cumul_intergral
}

func get_max(t []time.Time, y []uint64, start time.Time, end time.Time) uint64 {
	var max uint64
	for i := 0; i < len(t); i++ {
		if t[i].After(start) {
			for j := i; j < len(t); j++ {
				if t[j].Before(end) {
					if y[j] > max {
						max = y[j]
					}
					continue
				}
				break
			}
			break
		}
		continue
	}
	return max
}

func get_avg(t []time.Time, integral []float64, start time.Time, end time.Time) float64 {
	var avg float64
	var t_start, t_end time.Time
	for i := 0; i < len(t); i++ {
		if t[i].After(start) {
			avg = -integral[i]
			t_start = t[i]
			for j := i; j < len(t); j++ {
				if t[j].Before(end) {
					continue
				}
				avg += integral[j-1]
				t_end = t[j]
				break
			}
			break
		}
		continue
	}
	avg = avg / float64(t_end.Sub(t_start).Microseconds())
	return avg
}

func write_JSON_file(filepath string, data []byte) error {
	// Open a file for writing
	file, err := os.Create(filepath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		file.Close()
		return err
	}

	// Write the JSON data to the file
	_, err = file.Write(data)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
		return err
	}
	return nil
}

func write_CSV_file(filepath string, data [][]string) error {
	// Create the CSV file
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a new CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write data to CSV file
	for _, row := range data {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}
