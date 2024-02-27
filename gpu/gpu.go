package gpu

import (
	"log"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

const SAMPLIMG_PERIOD = 2

// Define a struct for GPU stats samples
type GPU_Sample struct {
	Util      nvml.Utilization
	Mem       nvml.Memory
	Pow       uint32
	Timestamp time.Time
}

func Init_NVML() {
	// Initialize NVML
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to initialize NVML: %v", nvml.ErrorString(ret))
	}
}

func Shutdown_NVML() {
	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to shutdown NVML: %v", nvml.ErrorString(ret))
	}
}

func Get_device(GPU_id int) nvml.Device {
	// Get device
	device, ret := nvml.DeviceGetHandleByIndex(int(GPU_id))
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to get device at index %d: %v", GPU_id, nvml.ErrorString(ret))
	}
	return device

}
func Gpu_sample(GPU nvml.Device) GPU_Sample {

	start := time.Now()
	// Get stats
	var sample GPU_Sample
	var ret nvml.Return
	sample.Util, ret = GPU.GetUtilizationRates()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to get utilization rates : %v", nvml.ErrorString(ret))
	}
	sample.Mem, ret = GPU.GetMemoryInfo()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to get memory stats : %v", nvml.ErrorString(ret))
	}
	sample.Pow, ret = GPU.GetPowerUsage()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to get power consumption : %v", nvml.ErrorString(ret))
	}
	sample.Timestamp = start
	//end := time.Now()
	//dur := end.Sub(start)
	//fmt.Printf("Utilization: %v\n", sample.util)
	//fmt.Printf("Memory: %v\n", sample.mem)
	//fmt.Printf("Power: %v\n", sample.pow)
	//fmt.Println("Query time in microseconds: ", dur.Microseconds())
	return sample

}
func GPU_Periodic_Samples(sampling_period uint64, GPU nvml.Device, stop <-chan struct{}, samples chan<- []GPU_Sample) {
	// Create a ticker that triggers every sampling period
	ticker := time.NewTicker(time.Duration(sampling_period * 1000000))
	defer ticker.Stop()
	// Create an unbounded slice to store GPU samples
	saved_samples := []GPU_Sample{}

	// Loop that runs until the program exits or the stop signal is received
	for {
		select {
		case <-ticker.C:
			// Periodically save samples
			saved_samples = append(saved_samples, Gpu_sample(GPU)) // Append to the unbounded slice
		case <-stop:
			// Signal to stop the sampling
			samples <- saved_samples // Send the unbounded slice as the result
			return
		}
	}
}
